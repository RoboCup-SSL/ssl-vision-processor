# scripts

Small utilities for developing and running vision_processor and its GUI host. Run them from anywhere; each prints
its usage for an argument it doesn't know.

| Script            | What it does                                                                                          |
| ----------------- | ----------------------------------------------------------------------------------------------------- |
| `lo-multicast.sh` | Checks (`status`), enables (`on`) or disables (`off`) multicast on loopback. Needs sudo to change it. |

## lo-multicast.sh

vision_processor and the GUI host exchange multicast packets. When both run on one machine, or a test sends and
receives on the same machine, those packets go over loopback, which Ubuntu ships with multicast off. The GUI host
warns about this at startup.

```
scripts/lo-multicast.sh            # status: exit 0 if on, 1 if off
scripts/lo-multicast.sh on         # enable, until the next reboot
scripts/lo-multicast.sh off        # back to Ubuntu's default
scripts/lo-multicast.sh status eth0
```

CI uses `on` before tests that send real multicast over loopback.
