#!/usr/bin/env python3
"""
Formats raw Go benchstat text output into a clean, executive GitHub Step Summary Markdown report.
"""
import sys
import re

def parse_val_unit(s):
    """Parses value string like '59.30m', '7.601Mi', '18.84k' into a numeric float."""
    s = s.strip()
    m = re.match(r'^([0-9.]+)([a-zA-Z%]*)$', s)
    if not m:
        return 0.0
    val_str, unit = m.groups()
    val = float(val_str)
    multiplier = 1.0
    if unit == 'k': multiplier = 1e3
    elif unit in ('M', 'Mi'): multiplier = 1e6
    elif unit in ('G', 'Gi'): multiplier = 1e9
    elif unit == 'm': multiplier = 1e-3
    elif unit in ('u', 'µ'): multiplier = 1e-6
    elif unit == 'n': multiplier = 1e-9
    return val * multiplier

def compute_pct(old_s, new_s):
    old_v = parse_val_unit(old_s)
    new_v = parse_val_unit(new_s)
    if old_v == 0:
        return 0.0
    return ((new_v - old_v) / old_v) * 100.0

def format_benchstat(text, pr_mode=False, run_url=None):
    lines = text.splitlines()
    current_metric = None
    geomeans = {}
    fixtures = {} # fix_name -> {'time': (old, new), 'mem': (old, new), 'allocs': (old, new)}

    for line in lines:
        line_str = line.strip()
        if 'sec/op' in line_str and 'vs base' in line_str:
            current_metric = 'time'
        elif 'B/op' in line_str and 'vs base' in line_str:
            current_metric = 'mem'
        elif 'allocs/op' in line_str and 'vs base' in line_str:
            current_metric = 'allocs'
        elif line_str.startswith('geomean'):
            parts = line_str.split()
            if len(parts) >= 3 and current_metric:
                delta = parts[3] if len(parts) >= 4 else "~"
                geomeans[current_metric] = (parts[1], parts[2], delta)
        elif '/' in line_str and not line_str.startswith(('geomean', 'goos:', 'goarch:', 'pkg:', 'cpu:')):
            parts = line_str.split()
            if len(parts) >= 3 and current_metric:
                fixture_raw = parts[0]
                fix_name = fixture_raw.split('/', 1)[1] if '/' in fixture_raw else fixture_raw
                if '-' in fix_name:
                    fix_name = fix_name.rsplit('-', 1)[0]
                
                vals = [p for p in parts[1:] if re.match(r'^[0-9.]+[a-zA-Z]*$', p)]
                if len(vals) >= 2:
                    old_v, new_v = vals[0], vals[1]
                    if fix_name not in fixtures:
                        fixtures[fix_name] = {}
                    fixtures[fix_name][current_metric] = (old_v, new_v)

    # Build Markdown Output
    md = []
    md.append("### 📊 Benchmark A/B Comparison Report")
    if run_url:
        md.append(f"> 🔗 [View Action Logs & Full Breakdown]({run_url})")
    md.append("")
    md.append("#### 📈 Key Metrics Summary (Geometric Mean)")
    md.append("| Metric | Baseline | Current | Delta | Status |")
    md.append("| :--- | :---: | :---: | :---: | :---: |")

    metric_meta = {
        'time': ('Execution Time', 'sec/op'),
        'mem': ('Memory Allocated', 'B/op'),
        'allocs': ('Allocations', 'allocs/op')
    }

    for key in ['time', 'mem', 'allocs']:
        if key in geomeans:
            old_g, new_g, _ = geomeans[key]
            pct = compute_pct(old_g, new_g)
            name, _ = metric_meta[key]
            
            if abs(pct) < 0.5:
                status = "➖ Neutral"
                badge = f"`{pct:+.2f}%`"
            elif pct < 0:
                status = "🟢 Faster" if key == 'time' else "🟢 Reduced"
                badge = f"**`{pct:+.2f}%`**"
            else:
                status = "🔴 Slower" if key == 'time' else "🟡 Increased"
                badge = f"**`{pct:+.2f}%`**"
                
            md.append(f"| **{name}** | `{old_g}` | `{new_g}` | {badge} | {status} |")

    md.append("")

    # Sort changes by time delta
    changes = []
    for fix_name, data in fixtures.items():
        if 'time' in data:
            old_t, new_t = data['time']
            pct = compute_pct(old_t, new_t)
            changes.append((fix_name, pct, old_t, new_t, data.get('mem', ('-', '-'))))

    changes.sort(key=lambda x: x[1])

    speedups = [c for c in changes if c[1] <= -1.0][:5]
    regressions = [c for c in changes if c[1] >= 1.0][:5]

    if speedups or regressions:
        md.append("#### ⚡ Notable Performance Shifts (Top 5)")
        md.append("| Fixture | Baseline Time | Current Time | Time Delta | Memory Delta |")
        md.append("| :--- | :---: | :---: | :---: | :---: |")
        
        for fix_name, pct, old_t, new_t, mem_tuple in speedups + regressions:
            icon = "🚀" if pct < 0 else "⚠️"
            old_m, new_m = mem_tuple
            mem_pct_str = "-"
            if old_m != '-' and new_m != '-':
                m_pct = compute_pct(old_m, new_m)
                mem_pct_str = f"`{m_pct:+.1f}%`"
            md.append(f"| {icon} `{fix_name}` | `{old_t}` | `{new_t}` | **`{pct:+.1f}%`** | {mem_pct_str} |")
        md.append("")

    if pr_mode:
        return "\n".join(md)

    # Collapsible Table for All Fixtures in full step summary
    md.append("<details>")
    md.append(f"<summary><b>🔍 Full Breakdown ({len(fixtures)} Fixtures)</b></summary>")
    md.append("")
    md.append("| Fixture | Baseline Time | Current Time | Time Delta | Baseline Mem | Current Mem | Mem Delta |")
    md.append("| :--- | :---: | :---: | :---: | :---: | :---: | :---: |")

    for fix_name, data in sorted(fixtures.items()):
        old_t, new_t = data.get('time', ('-', '-'))
        old_m, new_m = data.get('mem', ('-', '-'))
        
        t_pct_str = "`~`"
        if old_t != '-' and new_t != '-':
            t_pct = compute_pct(old_t, new_t)
            t_pct_str = f"`{t_pct:+.1f}%`" if abs(t_pct) >= 0.5 else "`~`"
            
        m_pct_str = "`~`"
        if old_m != '-' and new_m != '-':
            m_pct = compute_pct(old_m, new_m)
            m_pct_str = f"`{m_pct:+.1f}%`" if abs(m_pct) >= 0.5 else "`~`"

        md.append(f"| `{fix_name}` | `{old_t}` | `{new_t}` | {t_pct_str} | `{old_m}` | `{new_m}` | {m_pct_str} |")

    md.append("")
    md.append("</details>")
    md.append("")
    md.append("<details>")
    md.append("<summary><b>📄 Raw benchstat Console Output</b></summary>")
    md.append("")
    md.append("```")
    md.append(text.strip())
    md.append("```")
    md.append("</details>")

    return "\n".join(md)

if __name__ == '__main__':
    pr_mode = '--pr' in sys.argv
    run_url = None
    if '--run-url' in sys.argv:
        idx = sys.argv.index('--run-url')
        if idx + 1 < len(sys.argv):
            run_url = sys.argv[idx + 1]
    
    file_args = [a for a in sys.argv[1:] if a != '--pr' and a != '--run-url' and a != run_url]
    if file_args:
        with open(file_args[0], 'r') as f:
            content = f.read()
    else:
        content = sys.stdin.read()
    print(format_benchstat(content, pr_mode=pr_mode, run_url=run_url))
