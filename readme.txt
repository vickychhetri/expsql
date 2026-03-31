# 1. Basic parallel export with auto strategy selection
./mysqltool export-parallel \
  --host localhost \
  --port 3306 \
  --user root \
  --password "Sonika@1987" \
  --database funding_manage \
  --output ./backup \
  --workers 8

# 2. Force parallel export for all tables (good for very large tables)
./mysqltool export-parallel \
  --database funding_manage \
  --strategy parallel \
  --workers 8 \
  --partitions 16 \
  --rows-per-batch 50000

# 3. Streaming export for medium-large tables
./mysqltool export-parallel \
  --database funding_manage \
  --strategy streaming \
  --workers 4 \
  --rows-per-batch 25000

# 4. Resumable export (can be interrupted and resumed)
./mysqltool export-parallel \
  --database funding_manage \
  --strategy auto \
  --resumable true \
  --workers 6 \
  --output ./backup_resumable

# 5. Export only specific tables
./mysqltool export-parallel \
  --database funding_manage \
  --tables users,orders,products,sessions \
  --workers 4

# 6. Export large table with custom partition count
./mysqltool export-parallel \
  --database funding_manage \
  --tables large_table \
  --strategy parallel \
  --workers 8 \
  --partitions 32 \
  --rows-per-batch 100000 \
  --output ./backup_large

# 7. Export with compression for large datasets
./mysqltool export-parallel \
  --database funding_manage \
  --compress true \
  --workers 8 \
  --strategy parallel \
  --output ./backup_compressed

# 8. Design only export (no data)
./mysqltool export-parallel \
  --database funding_manage \
  --include-data false \
  --output ./design_only

# 9. Data only export for specific tables
./mysqltool export-parallel \
  --database funding_manage \
  --include-design false \
  --tables sessions,logs \
  --workers 4 \
  --output ./data_only