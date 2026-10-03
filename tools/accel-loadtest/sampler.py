import json, sys, time, urllib.request

# usage: sampler.py <exporter port> <seconds>
# Once a second: what the exporter says about the accelerators, plus the raw kernel
# view of the video engines (/proc/mpp_service/load is readable without root).
port, secs = sys.argv[1], int(sys.argv[2])


def mpp():
    out = {}
    for line in open("/proc/mpp_service/load"):
        p = line.split()
        if len(p) >= 5 and p[1] == "load:":
            out[p[0]] = [float(p[2].rstrip("%")), float(p[4].rstrip("%"))]
    return out


def by_label(m, name):
    res = {}
    for s in m.get(name, []):
        lab = s.get("labels") or {}
        res[lab.get("core") or lab.get("scheduler") or lab.get("unit") or ""] = s["value"]
    return res


end = time.time() + secs
nxt = int(time.time()) + 1
while time.time() < end:
    time.sleep(max(0, nxt - time.time()))
    t = nxt
    nxt += 1
    m = json.load(urllib.request.urlopen("http://127.0.0.1:%s/api/stats?view=live" % port, timeout=5))["metrics"]
    print(json.dumps({
        "t": t,
        "npu": by_label(m, "rk3588_npu_load_percent"),
        "rga": by_label(m, "rk3588_rga_load_percent"),
        "vpu": by_label(m, "rk3588_vpu_load_percent"),
        "vpu_util": by_label(m, "rk3588_vpu_utilization_percent"),
        "sessions": [s["value"] for s in m.get("rk3588_vpu_sessions", [])],
        "gpu": [s["value"] for s in m.get("rk3588_gpu_load_percent", [])],
        "mpp": mpp(),
    }), flush=True)
