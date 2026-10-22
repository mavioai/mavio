# Prepares SABnzbd's configuration before it starts: its API key, the
# container's name among the host names it answers to, and, on the first
# start, its folders in /downloads; and the categories of Mavio's movies
# and series.
import os
from urllib.parse import urlsplit

from configobj import ConfigObj

path = "/config/sabnzbd.ini"
first = not os.path.exists(path)
os.makedirs(os.path.dirname(path), exist_ok=True)
config = ConfigObj(path, encoding="utf-8")
misc = config.setdefault("misc", {})
misc["api_key"] = os.environ["MAVIO_API_KEY"]
hosts = misc.get("host_whitelist", [])
if isinstance(hosts, str):
    hosts = [h.strip() for h in hosts.split(",") if h.strip()]
name = urlsplit(os.environ["MAVIO_ADDRESS"]).hostname
if name and name not in hosts:
    hosts.append(name)
misc["host_whitelist"] = hosts
if first:
    misc["download_dir"] = "/downloads/incomplete"
    misc["complete_dir"] = "/downloads/complete"
categories = config.setdefault("categories", {})
for order, category in enumerate(["*", "movies-mavio", "tv-mavio"]):
    if category not in categories:
        default = category == "*"
        categories[category] = {
            "name": category,
            "order": str(order),
            "pp": "3" if default else "",
            "script": "None" if default else "Default",
            "dir": "",
            "priority": "0" if default else "-100",
        }
config.write()
