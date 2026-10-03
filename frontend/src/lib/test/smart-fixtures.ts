// /api/smart answers of the CM3588 test board (2026-10-03), serial numbers replaced, the SCT history cut to its last 24 samples.
// Only used by tests.

import type { SmartDetail } from '../api/types'

export const samsungSata: SmartDetail = {
  "device": "sda",
  "available": true,
  "read_at": 1790981809039,
  "smartctl": "7.4",
  "protocol": "ATA",
  "identity": {
    "model": "Samsung SSD 870 QVO 2TB",
    "family": "Samsung based SSDs",
    "firmware": "SVQ02B6Q",
    "serial": "TESTSERIAL0001",
    "wwn": "0x5002538000000001",
    "capacity_bytes": 2000398934016,
    "block_size": 512,
    "rotation_rpm": 0,
    "form_factor": "2.5 inches",
    "standard": "ACS-4 T13/BSR INCITS 529 revision 5",
    "interface": "SATA 3.3",
    "link_current": "6.0 Gb/s",
    "link_max": "6.0 Gb/s",
    "trim": true,
    "known_model": true
  },
  "health": {
    "state": "ok",
    "passed": true
  },
  "vitals": {
    "temperature_c": 34,
    "temp_lifetime_max_c": 84,
    "temp_lifetime_min_c": 24,
    "temp_limit_c": 70,
    "power_on_hours": 20396,
    "power_cycles": 107,
    "wear_used_pct": 0,
    "wear_source": "device statistics",
    "written_bytes": 4504376480256,
    "written_source": "device statistics",
    "read_bytes": 1975637997568,
    "read_source": "device statistics",
    "unsafe_shutdowns": 99,
    "unsafe_source": "attribute 235 POR_Recovery_Count",
    "reallocated": 0,
    "uncorrectable": 0,
    "crc_errors": 0,
    "levels": {
      "crc_errors": "ok",
      "reallocated": "ok",
      "uncorrectable": "ok",
      "wear": "ok"
    }
  },
  "attributes": [
    {
      "id": 5,
      "name": "Reallocated_Sector_Ct",
      "value": 100,
      "worst": 100,
      "thresh": 10,
      "raw": 0,
      "raw_text": "0",
      "prefail": true,
      "role": "reallocated",
      "critical": true,
      "level": "ok"
    },
    {
      "id": 9,
      "name": "Power_On_Hours",
      "value": 95,
      "worst": 95,
      "thresh": 0,
      "raw": 20396,
      "raw_text": "20396",
      "prefail": false,
      "role": "power_on_hours",
      "critical": false,
      "level": "ok"
    },
    {
      "id": 12,
      "name": "Power_Cycle_Count",
      "value": 99,
      "worst": 99,
      "thresh": 0,
      "raw": 107,
      "raw_text": "107",
      "prefail": false,
      "role": "power_cycles",
      "critical": false,
      "level": "ok"
    },
    {
      "id": 177,
      "name": "Wear_Leveling_Count",
      "value": 99,
      "worst": 99,
      "thresh": 0,
      "raw": 3,
      "raw_text": "3",
      "prefail": true,
      "role": "wear",
      "critical": true,
      "level": "ok"
    },
    {
      "id": 179,
      "name": "Used_Rsvd_Blk_Cnt_Tot",
      "value": 100,
      "worst": 100,
      "thresh": 10,
      "raw": 0,
      "raw_text": "0",
      "prefail": true,
      "critical": true,
      "level": "ok"
    },
    {
      "id": 181,
      "name": "Program_Fail_Cnt_Total",
      "value": 100,
      "worst": 100,
      "thresh": 10,
      "raw": 0,
      "raw_text": "0",
      "prefail": false,
      "critical": false,
      "level": "ok"
    },
    {
      "id": 182,
      "name": "Erase_Fail_Count_Total",
      "value": 100,
      "worst": 100,
      "thresh": 10,
      "raw": 0,
      "raw_text": "0",
      "prefail": false,
      "critical": false,
      "level": "ok"
    },
    {
      "id": 183,
      "name": "Runtime_Bad_Block",
      "value": 100,
      "worst": 100,
      "thresh": 10,
      "raw": 0,
      "raw_text": "0",
      "prefail": true,
      "critical": true,
      "level": "ok"
    },
    {
      "id": 187,
      "name": "Uncorrectable_Error_Cnt",
      "value": 100,
      "worst": 100,
      "thresh": 0,
      "raw": 0,
      "raw_text": "0",
      "prefail": false,
      "role": "uncorrectable",
      "critical": true,
      "level": "ok"
    },
    {
      "id": 190,
      "name": "Airflow_Temperature_Cel",
      "value": 66,
      "worst": 16,
      "thresh": 0,
      "raw": 34,
      "raw_text": "34",
      "prefail": false,
      "role": "temperature",
      "critical": false,
      "level": "ok"
    },
    {
      "id": 195,
      "name": "ECC_Error_Rate",
      "value": 200,
      "worst": 200,
      "thresh": 0,
      "raw": 0,
      "raw_text": "0",
      "prefail": false,
      "critical": false,
      "level": "ok"
    },
    {
      "id": 199,
      "name": "CRC_Error_Count",
      "value": 100,
      "worst": 100,
      "thresh": 0,
      "raw": 0,
      "raw_text": "0",
      "prefail": false,
      "role": "crc",
      "critical": true,
      "level": "ok"
    },
    {
      "id": 235,
      "name": "POR_Recovery_Count",
      "value": 99,
      "worst": 99,
      "thresh": 0,
      "raw": 99,
      "raw_text": "99",
      "prefail": false,
      "role": "unsafe_shutdown",
      "critical": false,
      "level": "info"
    },
    {
      "id": 241,
      "name": "Total_LBAs_Written",
      "value": 99,
      "worst": 99,
      "thresh": 0,
      "raw": 8797610313,
      "raw_text": "8797610313",
      "prefail": false,
      "role": "written",
      "critical": false,
      "level": "ok",
      "bytes": 4504376480256
    }
  ],
  "logs": {
    "error_entries": 0,
    "self_test": {
      "count": 0,
      "short_minutes": 2,
      "extended_minutes": 160
    }
  },
  "temp_history": {
    "interval_minutes": 10,
    "samples": [
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34,
      34
    ]
  }
}

export const lexarNvme: SmartDetail = {
  "device": "nvme0n1",
  "available": true,
  "read_at": 1790981808741,
  "smartctl": "7.4",
  "protocol": "NVMe",
  "identity": {
    "model": "Lexar SSD NM790 1TB",
    "firmware": "11296",
    "serial": "TESTSERIAL0001",
    "capacity_bytes": 1024209543168,
    "block_size": 512,
    "standard": "NVMe 1.4",
    "link_current": "PCIe Gen3 ×1 · 8.0 GT/s",
    "link_max": "PCIe Gen4 ×4 · 16.0 GT/s"
  },
  "health": {
    "state": "ok",
    "passed": true
  },
  "vitals": {
    "temperature_c": 38,
    "temp_limit_c": 89.85,
    "temp_critical_c": 94.85,
    "power_on_hours": 7248,
    "power_cycles": 189,
    "wear_used_pct": 0,
    "wear_source": "NVMe health log",
    "spare_remaining_pct": 100,
    "spare_threshold_pct": 10,
    "written_bytes": 6548747776000,
    "written_source": "NVMe health log",
    "read_bytes": 37495296000,
    "read_source": "NVMe health log",
    "unsafe_shutdowns": 104,
    "unsafe_source": "NVMe health log",
    "media_errors": 0,
    "levels": {
      "media_errors": "ok",
      "spare": "ok",
      "wear": "ok"
    }
  },
  "nvme": {
    "critical_warning": 0,
    "available_spare": 100,
    "available_spare_threshold": 10,
    "percentage_used": 0,
    "data_units_read": 73233,
    "data_units_written": 12790523,
    "host_reads": 1445320,
    "host_writes": 113952962,
    "controller_busy_minutes": 303,
    "power_cycles": 189,
    "power_on_hours": 7248,
    "unsafe_shutdowns": 104,
    "media_errors": 0,
    "error_log_entries": 0,
    "warning_temp_minutes": 0,
    "critical_temp_minutes": 0,
    "sensors": [
      38,
      32
    ],
    "levels": {
      "unsafe_shutdowns": "info"
    }
  },
  "logs": {
    "error_entries": 0
  }
}

