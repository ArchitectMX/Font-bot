import cv2
import numpy as np
import os
import shutil
from tqdm import tqdm
import sys
import argparse
from corrector import process_image_cv2


def prepare_output_folder(folder="storage/Image"):
    """Очистка и создание выходной папки"""
    if os.path.exists(folder):
        shutil.rmtree(folder, ignore_errors=True)
    os.makedirs(folder, exist_ok=True)


def load_image(image_path):
    """Загрузка изображения с проверкой"""
    image = cv2.imread(image_path)
    if image is None:
        raise ValueError(f"Failed to load image: {image_path}")
    return image, cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)


def get_largest_contour(gray):
    """Поиск основного контура с улучшенной бинаризацией"""
    blurred = cv2.GaussianBlur(gray, (5, 5), 0)
    thresh = cv2.adaptiveThreshold(
        blurred, 255,
        cv2.ADAPTIVE_THRESH_GAUSSIAN_C,
        cv2.THRESH_BINARY_INV, 21, 6
    )
    contours, _ = cv2.findContours(thresh, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    return max(contours, key=cv2.contourArea) if contours else None


def rotate_and_align(image, contour):
    """Улучшенная версия с правильным определением угла поворота"""
    rect = cv2.minAreaRect(contour)
    (cx, cy), (w, h), angle = rect

    if angle < -45:
        angle += 90
        w, h = h, w

    elif abs(angle) > 45:
        angle += 90 if angle < 0 else -90
        w, h = h, w

    expand_pixels = 20
    matrix = cv2.getRotationMatrix2D((cx, cy), angle, 1.0)

    new_w = int((h * np.sin(np.deg2rad(abs(angle))) + w * np.cos(np.deg2rad(abs(angle)))) + expand_pixels)
    new_h = int((h * np.cos(np.deg2rad(abs(angle))) + w * np.sin(np.deg2rad(abs(angle)))) + expand_pixels)

    matrix[0, 2] += (new_w / 2 - cx)
    matrix[1, 2] += (new_h / 2 - cy)

    aligned = cv2.warpAffine(
        image, matrix, (new_w, new_h),
        flags=cv2.INTER_CUBIC,
        borderMode=cv2.BORDER_CONSTANT,
        borderValue=(255, 255, 255)
    )

    gray = cv2.cvtColor(aligned, cv2.COLOR_BGR2GRAY)
    _, thresh = cv2.threshold(gray, 0, 255, cv2.THRESH_BINARY_INV + cv2.THRESH_OTSU)
    contours, _ = cv2.findContours(thresh, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)

    if contours:
        largest = max(contours, key=cv2.contourArea)
        x, y, w, h = cv2.boundingRect(largest)
        return aligned[y:y + h, x:x + w]

    return aligned


def get_unicode_filename(char):
    return f"U+{ord(char):04X}"


def normalize_image_heights(output_dir):
    """Выравнивание высоты всех сохраненных изображений до максимальной."""
    file_paths = [os.path.join(output_dir, f) for f in os.listdir(output_dir) if f.endswith('.png')]
    heights = [cv2.imread(fp).shape[0] for fp in file_paths]

    if not heights:
        return

    max_height = max(heights)

    for file_path, height in zip(file_paths, heights):
        if height < max_height:
            img = cv2.imread(file_path)
            padding = max_height - height
            pad_top = np.full((padding, img.shape[1], 3), 255, dtype=np.uint8)
            padded_img = np.vstack((pad_top, img))
            cv2.imwrite(file_path, padded_img)


def extract_grid_cells(image, rows=8, cols=9, output_dir="cells"):
    """Финальная версия с чёрными буквами на белом фоне и точной обрезкой"""
    processed = image.copy()

    letters = ["А", "а", "Б", "б", "В", "в", "Г", "г", "Д", "д", "Е", "е", "Ё", "ё", "Ж", "ж", "З", "з",
                   "И", "и", "Й", "й", "К", "к", "Л", "л", "М", "м", "Н", "н", "О", "о", "П", "п", "Р", "р", "С", "с",
                   "Т", "т", "У", "у", "Ф", "ф", "Х", "х", "Ц", "ц", "Ч", "ч", "Ш", "ш", "Щ", "щ", "Ъ", "ъ", "Ы", "ы",
                   "Ь", "ь", "Э", "э", "Ю", "ю", "Я", "я"]

    mask = np.zeros_like(processed)
    gray = cv2.cvtColor(processed, cv2.COLOR_BGR2GRAY)
    edges = cv2.Canny(gray, 80, 150, apertureSize=3)
    lines = cv2.HoughLinesP(edges, 1, np.pi / 180, 150, minLineLength=100, maxLineGap=5)

    vertical, horizontal = [], []
    if lines is not None:
        for line in lines:
            x1, y1, x2, y2 = line[0]
            dx, dy = abs(x1 - x2), abs(y1 - y2)
            if dx <= 2 and dy > 50:
                vertical.append((x1, x2))
                cv2.line(mask, (x1, y1), (x2, y2), (255, 255, 255), 3)
            elif dy <= 2 and dx > 50:
                horizontal.append((y1, y2))
                cv2.line(mask, (x1, y1), (x2, y2), (255, 255, 255), 3)

    mask_gray = cv2.cvtColor(mask, cv2.COLOR_BGR2GRAY)
    cleaned = cv2.inpaint(processed, mask_gray, 7, cv2.INPAINT_TELEA)
    cleaned = cv2.morphologyEx(cleaned, cv2.MORPH_OPEN, np.ones((3, 3), np.uint8), iterations=1)

    def cluster_grid(points, axis_max, expected):
        sorted_points = sorted({(p1 + p2) // 2 for p1, p2 in points})
        return [0] + sorted_points + [axis_max] if len(sorted_points) >= expected - 1 else \
            [int(i) for i in np.linspace(0, axis_max, expected + 1)]

    vertical_x = cluster_grid(vertical, cleaned.shape[1], cols)
    horizontal_y = cluster_grid(horizontal, cleaned.shape[0], rows)

    os.makedirs(output_dir, exist_ok=True)
    cell_counter = 0

    with tqdm(total=rows * cols, desc="Processing cells", unit="cell") as pbar:
        for i in range(len(horizontal_y) - 1):
            for j in range(len(vertical_x) - 1):
                try:
                    y1 = horizontal_y[i] + 4
                    y2 = horizontal_y[i + 1] - 4
                    x1 = vertical_x[j] + 4
                    x2 = vertical_x[j + 1] - 4
                    if (y2 - y1 < 10) or (x2 - x1 < 10):
                        continue

                    cell = cleaned[y1:y2, x1:x2].copy()

                    gray_cell = cv2.cvtColor(cell, cv2.COLOR_BGR2GRAY)
                    _, thresh = cv2.threshold(gray_cell, 0, 255, cv2.THRESH_BINARY + cv2.THRESH_OTSU)

                    contours, _ = cv2.findContours(thresh, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
                    if contours:
                        combined = np.vstack(contours)
                        x, y, w, h = cv2.boundingRect(combined)
                        pad = int(max(w, h) * 0.1)
                        x, y = max(0, x - pad), max(0, y - pad)
                        w, h = min(cell.shape[1] - x, w + 2 * pad), min(cell.shape[0] - y, h + 2 * pad)
                        cropped = cell[y:y + h, x:x + w]
                    else:
                        cropped = cell

                    gray_cropped = cv2.cvtColor(cropped, cv2.COLOR_BGR2GRAY)
                    _, binary = cv2.threshold(gray_cropped, 0, 255, cv2.THRESH_BINARY + cv2.THRESH_OTSU)

                    if np.mean(binary) < 127:
                        binary = cv2.bitwise_not(binary)

                    result = cv2.cvtColor(binary, cv2.COLOR_GRAY2BGR)

                    if cell_counter < len(letters):
                        filename = f"{get_unicode_filename(letters[cell_counter])}.png"

                        processed_img = process_image_cv2(result)
                        if processed_img is not None:
                            cv2.imwrite(os.path.join(output_dir, filename), processed_img)

                    cell_counter += 1
                    pbar.update(1)

                except Exception as e:
                    print(f"\nError in cell {cell_counter}: {str(e)}")
                    continue

    normalize_image_heights(output_dir)

    return output_dir


def process_image(input_path, output_folder="storage/Image", dpi_factor=1, grid_rows=8, grid_cols=9):
    """Основной процесс обработки с разделением на ячейки"""
    prepare_output_folder(output_folder)

    try:
        image, gray = load_image(input_path)
        contour = get_largest_contour(gray)

        if contour is None:
            raise ValueError("Не найдено подходящих контуров")

        aligned = rotate_and_align(image, contour)
        aligned_gray = cv2.cvtColor(aligned, cv2.COLOR_BGR2GRAY)

        _, enhanced = cv2.threshold(aligned_gray, 0, 255, cv2.THRESH_BINARY_INV + cv2.THRESH_OTSU)
        enhanced_color = cv2.cvtColor(enhanced, cv2.COLOR_GRAY2BGR)

        final_path = os.path.join(output_folder, "processed.png")
        cv2.imwrite(final_path, enhanced_color)

        cells_dir = os.path.join(output_folder, "cells")
        extract_grid_cells(enhanced_color, rows=grid_rows, cols=grid_cols, output_dir=cells_dir)

        print(f"Обработка завершена. Результаты в: {output_folder}")
        return final_path

    except Exception as e:
        shutil.rmtree(output_folder, ignore_errors=True)
        raise RuntimeError(f"Ошибка обработки: {e}") from e


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Обработчик изображений')
    parser.add_argument('image_path', help='Путь к исходному изображению')
    parser.add_argument('--dpi', type=float, default=1, help='Фактор DPI')
    parser.add_argument('--rows', type=int, default=8, help='Количество строк сетки')
    parser.add_argument('--cols', type=int, default=9, help='Количество столбцов сетки')

    args = parser.parse_args()

    try:
        result = process_image(
            args.image_path,
            dpi_factor=args.dpi,
            grid_rows=args.rows,
            grid_cols=args.cols
        )
        print(f"SUCCESS: {result}")
    except Exception as e:
        print(f"ERROR: {str(e)}")
        sys.exit(1)


