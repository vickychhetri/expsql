# Full export with 8 workers
./mysqltool export --host localhost --port 3306 --user root --password mypass \
  --database mydb --output ./backup --workers 8 --rows-per-batch 50000

# Export only design (no data)
./mysqltool export --database mydb --include-data=false

# Export only specific tables
./mysqltool export --database mydb --tables users,orders,products --workers 4

# Exclude certain tables
./mysqltool export --database mydb --exclude-tables logs,audit --workers 4

# Compress output
./mysqltool export --database mydb --compress --workers 8