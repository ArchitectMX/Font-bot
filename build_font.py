import subprocess
import os
import time

fontforge_bat = os.path.join("FontForgeBuilds", "fontforge-console.bat")
script_path = os.path.join("import_svg.py")

if not os.path.exists(fontforge_bat):
    print("Ошибка: fontforge-console.bat не найден. Проверьте путь к FontForge!")
    exit(1)

if not os.path.exists(script_path):
    print(f"Ошибка: {script_path} не найден. Проверьте путь к скрипту!")
    exit(1)

try:
    process = subprocess.Popen(
        fontforge_bat,
        shell=True,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        encoding="utf-8",
        errors="replace"
    )

    time.sleep(3)

    process.stdin.write(f'ffpython "{script_path}"\n')
    process.stdin.write('exit\n')
    process.stdin.flush()

    stdout, stderr = process.communicate()
    print("=== Вывод FontForge ===")
    print(stdout)
    print("========================")

    if process.returncode == 0:
        print("FontForge успешно сгенерировал TTF!")
    else:
        print("Ошибка при выполнении FontForge!")
        print(stderr)

except Exception as e:
    print(f"Ошибка запуска FontForge: {e}")
