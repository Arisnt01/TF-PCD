import csv

INPUT = "ratings.csv"
PARTS = 4

# Contar filas
with open(INPUT, "r", encoding="utf-8") as f:
    total_rows = sum(1 for _ in f) - 1  # sin header

chunk = total_rows // PARTS

print(f"Total filas (sin header): {total_rows}")
print(f"Filas por parte: {chunk}")

# Leer todo
with open(INPUT, "r", encoding="utf-8") as f:
    reader = csv.reader(f)
    header = next(reader)
    rows = list(reader)

start = 0
for i in range(1, PARTS + 1):
    end = start + chunk
    # última parte se lleva el resto
    if i == PARTS:
        end = total_rows

    part_file = f"ratings_part{i}.csv"
    with open(part_file, "w", newline="", encoding="utf-8") as out:
        w = csv.writer(out)
        w.writerow(header)
        w.writerows(rows[start:end])

    print(f"Creado → {part_file} con {end-start} filas")
    start = end
