import os
import subprocess
import uuid
import cv2
import numpy as np
from pathlib import Path
from PIL import Image
from svg2png import svg2png

# Пути
potrace_path = "potrace/potrace.exe"


def process_image_cv2(result):
    """
    Обрабатывает изображение (numpy array) через potrace и конвертирует обратно в OpenCV-формат.

    Параметры:
    - result: np.array (изображение в формате OpenCV, например, после cv2.cvtColor)

    Возвращает:
    - processed_result: np.array (обработанное изображение)
    """
    temp_dir = "storage/Image/temp"
    Path(temp_dir).mkdir(parents=True, exist_ok=True)

    unique_id = uuid.uuid4().hex  # Уникальный идентификатор
    temp_svg = os.path.join(temp_dir, f"temp_{unique_id}.svg")
    bmp_path = os.path.join(temp_dir, f"temp_{unique_id}.bmp")

    try:
        # Преобразуем OpenCV-изображение в Pillow и сохраняем как BMP
        pil_img = Image.fromarray(result)
        pil_img = pil_img.convert("L").point(lambda x: 255 if x > 200 else 0, mode='1')
        pil_img.save(bmp_path, format="BMP")

        if not os.path.exists(bmp_path):
            raise FileNotFoundError(f"Ошибка: BMP-файл {bmp_path} не был создан.")

        # Запускаем potrace для конвертации BMP в SVG
        subprocess.run(
            [
                potrace_path,
                bmp_path,
                "-b", "svg",
                "-o", temp_svg,
                "--opttolerance", "0.1",
                "--alphamax", "1",
                "--tight"
            ],
            check=True,
            timeout=30,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE
        )

        if not os.path.exists(temp_svg):
            raise FileNotFoundError(f"Ошибка: SVG-файл {temp_svg} не был создан.")

        # Конвертируем SVG в PNG
        svg2png(temp_svg)

        # Определяем путь к PNG-файлу (он будет создан рядом с SVG)
        temp_png = temp_svg.replace(".svg", ".png")

        if not os.path.exists(temp_png):
            raise FileNotFoundError(f"Ошибка: PNG-файл {temp_png} не был создан после конверсии.")

        processed_result = cv2.imread(temp_png, cv2.IMREAD_UNCHANGED)

        if processed_result is None:
            raise ValueError("Ошибка: OpenCV не смог загрузить обработанный PNG-файл.")

        return processed_result

    except subprocess.CalledProcessError as e:
        print(f"Ошибка выполнения potrace: {e}")
        print(f"STDOUT: {e.stdout.decode('utf-8')}")
        print(f"STDERR: {e.stderr.decode('utf-8')}")

    except Exception as e:
        print(f"Ошибка обработки изображения: {e}")

    finally:
        # Удаляем временные файлы
        for temp_file in [bmp_path, temp_svg, temp_png]:
            if os.path.exists(temp_file):
                try:
                    os.remove(temp_file)
                except Exception as ex:
                    print(f"Ошибка удаления {temp_file}: {ex}")

    return None

