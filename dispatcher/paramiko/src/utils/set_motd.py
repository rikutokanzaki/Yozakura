import os
import datetime

def get_motd_lines(hostname):
  motd_file_path = os.path.join(os.path.dirname(__file__), "/config/motd.txt")

  now = datetime.datetime.now(datetime.timezone.utc).strftime("%a %b %d %H:%M:%S UTC %Y")

  formatted_hostname = (hostname + ":").ljust(10)

  try:
    with open(motd_file_path, "r", encoding="utf-8") as f:
      lines = f.readlines()

    return [line.format(now=now, hostname=formatted_hostname) for line in lines]
  except Exception as e:
    print(f"Failed to read motd file: {e}")
    fallback_message = f"Welcome. (Host: 192.168.100.3 Time: {now})"
    return [fallback_message]
