"""Compares sample.jsonl (what the exporter reported each second) with raw.log (the kernel's own NPU and RGA
load files) and with /proc/mpp_service/load (VPU), one line per second while something was busy."""
import json, re

phases = [(int(a), b) for a, b in (l.split() for l in open("phases.log"))]


def phase_of(t):
    cur = "-"
    for ts, name in phases:
        if t >= ts:
            cur = name
    return cur


raw, cur = [], None
for line in open("raw.log"):
    if line.startswith("T "):
        cur = {"t": int(line.split()[1]), "npu": None, "rga": []}
        raw.append(cur)
    elif cur is not None:
        m = re.search(r"Core0:\s*(\d+)%,\s*Core1:\s*(\d+)%,\s*Core2:\s*(\d+)%", line)
        if m:
            cur["npu"] = [int(x) for x in m.groups()]
        m = re.search(r"load = (\d+)%", line)
        if m:
            cur["rga"].append(int(m.group(1)))

rows = [json.loads(l) for l in open("sample.jsonl")]
t0 = rows[0]["t"]
print("   t phase       | NPU exporter  | kernel NPU in (t-1..t) | RGA exporter | VPU enc0/enc1/dec0/dec1 | mpp enc/dec         | sessions")
bad = 0
for r in rows:
    npu = [r["npu"].get(k) for k in ("0", "1", "2")]
    rga = [r["rga"].get(k) for k in ("rga3_0", "rga3_1", "rga2_2")]
    v = r["vpu"]
    if not any(npu) and not any(rga) and not any(v.values()):
        continue
    near = [x for x in raw if r["t"] - 1 <= x["t"] <= r["t"]]
    seen = [x["npu"] for x in near]
    seen_rga = [x["rga"] for x in near]
    ok = (npu in seen or not seen) and (rga in seen_rga or not seen_rga)
    bad += 0 if ok else 1
    mpp = r["mpp"]
    enc = [mpp[k][0] for k in sorted(mpp) if "rkvenc" in k]
    dec = [mpp[k][0] for k in sorted(mpp) if "rkvdec" in k]
    print("%4d %-11s | %-13s | %-22s | %-12s | %s %s %s %s | %s %s | %s %s" % (
        r["t"] - t0, phase_of(r["t"]), npu, seen[-2:], rga, v.get("enc_core0"), v.get("enc_core1"), v.get("dec_core0"),
        v.get("dec_core1"), enc, dec, r["sessions"], "" if ok else "  <-- differs from the kernel file"))
print("\nseconds where the exporter's NPU or RGA load was not among the kernel's readings: %d" % bad)
