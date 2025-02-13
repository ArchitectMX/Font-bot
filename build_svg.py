import os
import subprocess
from pathlib import Path
from PIL import Image
import uuid

input_dir = "storage/Image/cells"
output_dir = "storage/svgStorage"
potrace_path = "potrace/potrace.exe"

Path(output_dir).mkdir(parents=True, exist_ok=True)

def convert_png_to_svg():
    for filename in os.listdir(input_dir):
        if not filename.lower().endswith(".png"):
            continue

        input_png = os.path.join(input_dir, filename)
        output_svg = os.path.join(output_dir, f"{Path(filename).stem}.svg")
        bmp_path = os.path.join(output_dir, f"temp_{uuid.uuid4().hex}.bmp")

        try:
            with Image.open(input_png) as img:
                img = img.convert("L").point(lambda x: 255 if x > 200 else 0, mode='1')
                img.save(bmp_path, format="BMP")

            subprocess.run(
                [
                    potrace_path,
                    bmp_path,
                    "-b", "svg",
                    "-o", output_svg,
                    "--opttolerance", "0.1",
                    "--alphamax", "1",
                ],
                check=True,
                timeout=30,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL
            )

        except Exception as e:
            print(f"Ошибка обработки {filename}: {e}")
        finally:
            if os.path.exists(bmp_path):
                try:
                    os.remove(bmp_path)
                except:
                    pass

if __name__ == "__main__":
    convert_png_to_svg()
