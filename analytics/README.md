# Аналитика: госпаблики vs обычные сообщества

`audience_report.ipynb` — отчёт по выгрузке из `vk-parser/data`: вовлечённость по
размеру сообществ, сравнение госпабликов с обычными, аудитория и её ядро.

## Запуск

```bash
python3 -m venv .venv
.venv/bin/pip install -r requirements.txt      # нужен доступ к PyPI
.venv/bin/jupyter lab audience_report.ipynb    # или: code audience_report.ipynb
```

Прогон целиком без открытия ноутбука:

```bash
.venv/bin/jupyter nbconvert --execute --to notebook --inplace audience_report.ipynb
```

## Чистовые данные

Поменять единственную строку в ячейке конфига:

```python
DATA = Path('../vk-parser/data')   # ← путь к чистовой выгрузке
```

и удалить `cache/` (там лежат агрегаты предыдущего прогона).

## is_gov

В `vk-parser/data/wall_owners.csv` добавлен бинарный столбец **`is_gov`** (сейчас
у всех `0`). Проставьте единицы госпабликам — все гос/не-гос графики оживут сами,
код менять не нужно. Исходник без столбца сохранён как `wall_owners.raw.csv`.

## Результаты

- `figures/` — PNG всех графиков
- `tables/` — сводные CSV (`summary.csv`, `by_size_bucket.csv`, `groups.csv`, `top_groups.csv`)
- `cache/` — агрегаты в parquet (чтобы не перечитывать 650 МБ `posts.csv` каждый раз)
