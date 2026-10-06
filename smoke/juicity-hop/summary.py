import json
import os
from pathlib import Path
import re


def required_int(name, minimum=1, maximum=65535):
    value = int(os.environ[name])
    if value < minimum or value > maximum:
        raise ValueError(f"{name} is outside its valid range")
    return value


base_port = required_int("BASE_PORT")
hop_start = required_int("HOP_START")
hop_end = required_int("HOP_END")
capture_ipv4 = os.environ["CAPTURE_SERVER_IPV4"]

rows = [
    json.loads(line)
    for line in Path("results.jsonl").read_text(encoding="utf-8").splitlines()
    if line.startswith("{")
]
packet_pattern = re.compile(
    rf"(?P<timestamp>\d+\.\d+).*?\bIP "
    rf"(?P<src_ip>\d+(?:\.\d+){{3}})\.(?P<src_port>\d+) > "
    rf"{re.escape(capture_ipv4)}\.(?P<dst_port>\d+):"
)
packets = []
for line in Path("ports.log").read_text(encoding="utf-8").splitlines():
    match = packet_pattern.search(line)
    if match:
        packets.append(
            (
                float(match["timestamp"]),
                match["src_ip"],
                int(match["src_port"]),
                int(match["dst_port"]),
            )
        )

if len(rows) != 4:
    raise AssertionError(f"expected baseline and three hop rounds, got {len(rows)}")

summary = []
for row in rows:
    sockets = {}
    for timestamp, src_ip, src_port, dst_port in packets:
        if row["start"] <= timestamp <= row["end"]:
            sockets.setdefault((src_ip, src_port), []).append(dst_port)

    destinations = sorted({port for sequence in sockets.values() for port in sequence})
    if row["mode"] == "baseline":
        if row["tcp_bytes"] != 131072 or row["udp_echo_ok"] != 4:
            raise AssertionError("fixed-port traffic did not meet TCP/UDP baseline")
        if destinations != [base_port]:
            raise AssertionError(f"fixed-port baseline used unexpected ports: {destinations}")
    else:
        if row["tcp_bytes"] != 1310720 or row["udp_echo_ok"] != 40:
            raise AssertionError(f"hop round {row['round']} did not meet TCP/UDP traffic criteria")
        if not destinations or any(not hop_start <= port <= hop_end for port in destinations):
            raise AssertionError(f"hop round used a port outside its verified range: {destinations}")

        rotating_sockets = []
        for source, sequence in sockets.items():
            changes = sum(left != right for left, right in zip(sequence, sequence[1:]))
            distinct = len(set(sequence))
            if distinct >= 4 and changes >= 3:
                rotating_sockets.append((source, distinct, changes))
        if not rotating_sockets:
            raise AssertionError("no single UDP socket showed at least four ports and three rotations")
        row["rotating_socket_count"] = len(rotating_sockets)

    row["destination_ports"] = destinations
    row["max_distinct_ports_per_socket"] = max(
        (len(set(sequence)) for sequence in sockets.values()), default=0
    )
    summary.append(row)

Path("summary.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
print(json.dumps(summary, indent=2))
