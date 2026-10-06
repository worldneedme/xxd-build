import json
from pathlib import Path
import re

rows = [json.loads(line) for line in Path('results.jsonl').read_text().splitlines() if line.startswith('{"')]
packets = []
for line in Path('ports.log').read_text().splitlines():
    m = re.match(r'(\d+\.\d+).*?\bIP [\d.]+\.(\d+) > 69\.33\.211\.38\.(\d+):', line)
    if m:
        packets.append((float(m[1]), int(m[2]), int(m[3])))
assert len(rows) == 4, f'expected baseline and three hop rounds, got {len(rows)}'
summary = []
for row in rows:
    flows = {}
    for t, source, port in packets:
        if row['start'] <= t <= row['end']:
            flows.setdefault(source, []).append(port)
    ports = sorted({p for seq in flows.values() for p in seq})
    if row['mode'] == 'baseline':
        assert ports == [10444], f'baseline used unexpected destination ports {ports}'
    else:
        assert ports and all(31000 <= p <= 31020 for p in ports), f'ports outside isolated range: {ports}'
        assert any(len(set(seq)) >= 4 and sum(a != b for a,b in zip(seq, seq[1:])) >= 3 for seq in flows.values()), 'fewer than three changes on one UDP socket'
        assert row['tcp_bytes'] == 80 * 16384 and row['udp_echo_ok'] == 40
    row['destination_ports'] = ports
    row['max_distinct_ports_per_socket'] = max(map(lambda seq: len(set(seq)), flows.values()))
    summary.append(row)
Path('summary.json').write_text(json.dumps(summary, indent=2))
print(json.dumps(summary, indent=2))
