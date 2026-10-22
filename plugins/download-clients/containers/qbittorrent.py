# Sets qBittorrent's Web UI before it starts: the user, the password's
# PBKDF2 hash, and no Host header validation, since the server reaches it
# at a published port other than its own. They go in its configuration's
# [WebUI] section, where qBittorrent moves them, or under [Preferences] in
# the default configuration the image copies on the first start.
import base64
import hashlib
import os

path = "/config/qBittorrent/qBittorrent.conf"
if not os.path.exists(path):
    path = "/defaults/qBittorrent.conf"
salt = os.urandom(16)
key = hashlib.pbkdf2_hmac("sha512", os.environ["MAVIO_PASSWORD"].encode(), salt, 100000, 64)
values = {
    "Username": os.environ["MAVIO_USERNAME"],
    "Password_PBKDF2": '"@ByteArray(%s:%s)"' % (base64.b64encode(salt).decode(), base64.b64encode(key).decode()),
    "HostHeaderValidation": "false",
}
with open(path, encoding="utf-8") as f:
    lines = f.read().splitlines()
section, prefix = ("[WebUI]", "") if "[WebUI]" in lines else ("[Preferences]", "WebUI\\")
names = {prefix + k for k in values} | {"WebUI\\" + k for k in values}
lines = [line for line in lines if line.split("=", 1)[0] not in names]
if section not in lines:
    lines.append(section)
i = lines.index(section) + 1
lines[i:i] = ["%s%s=%s" % (prefix, k, v) for k, v in values.items()]
with open(path, "w", encoding="utf-8") as f:
    f.write("\n".join(lines) + "\n")
