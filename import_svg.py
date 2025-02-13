import fontforge
import os
import pathlib
import psMat

fontname = "Echoing Ink"
svg_dir = "storage/svgStorage"
output_dir = "storage/fontStorage"
output_path = os.path.join(output_dir, f"{fontname}.ttf")

os.makedirs(output_dir, exist_ok=True)

font = fontforge.font()
font.encoding = 'UnicodeFull'
font.version = '1.0'
font.weight = 'Regular'
font.fontname = fontname
font.familyname = fontname
font.fullname = fontname

# Открываем Times New Roman как fallback-шрифт
fallback_font = fontforge.open("C:/Windows/Fonts/times.ttf")

# Импортируем SVG-глифы и сохраняем их в отдельный список
svg_glyphs = []
for p in pathlib.Path(svg_dir).glob("*.svg"):
    try:
        unicode_hex = p.stem.split(" ", 1)[0]
        unicode_index = int(unicode_hex[2:], 16)
        glyph = font.createChar(unicode_index)
        glyph.importOutlines(str(p))
        svg_glyphs.append(glyph)
    except Exception as e:
        print(f"Error importing {p}: {e}")

# Добавляем недостающие символы из Times New Roman (fallback)
for unicode_index in range(32, 128):
    if unicode_index not in font:
        new_glyph = font.createChar(unicode_index)
        if unicode_index in fallback_font:
            temp_path = os.path.join(output_dir, f"temp_{unicode_index}.svg")
            fallback_font[unicode_index].export(temp_path)
            new_glyph.importOutlines(temp_path)
            os.remove(temp_path)

# Применяем вертикальное смещение только к SVG-глифам
baseline_offset = 200  # Подберите нужное значение
for glyph in svg_glyphs:
    glyph.transform(psMat.translate(0, baseline_offset))

# Корректируем метрики шрифта (если необходимо)
font.ascent += baseline_offset
if hasattr(font, 'os2_typoascent'):
    font.os2_typoascent += baseline_offset
if hasattr(font, 'os2_typodescent'):
    font.os2_typodescent = max(0, font.os2_typodescent - baseline_offset)

font.generate(output_path)
print(f"Font saved to {output_path}")
