def find_password():
    import subprocess
    import re
    cmd = ['security', 'find-internet-password', '-gs', 'github.com']
    pwinfo = subprocess.Popen(cmd, stdout = subprocess.PIPE,
            stderr = subprocess.PIPE)
    pwline = pwinfo.stderr.read().decode('utf-8').strip()
    return re.sub('password: "(.*)"', '\\1', pwline)


PASSWORD = find_password()
