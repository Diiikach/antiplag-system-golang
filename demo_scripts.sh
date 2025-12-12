#!/bin/bash
# Демонстрационный скрипт для тестирования антиплагиата
set -e

# Файл для отправки
TESTDIR="demo_test_files"
mkdir -p $TESTDIR

echo "Мама мыла раму" > $TESTDIR/ivanov_hw1.txt
echo "Мама мыла раму" > $TESTDIR/petrov_hw1.txt
echo "Совсем другой текст" > $TESTDIR/sidorov_hw1.txt

echo "--- 1. Иванов сдаёт оригинал (hw1) ---"
curl -F "file=@$TESTDIR/ivanov_hw1.txt" -F "sender=Иванов" -F "work_id=hw1" http://localhost:8000/api/submit

echo -e "\n--- 2. Петров сдаёт такую же работу (hw1) ---"
curl -F "file=@$TESTDIR/petrov_hw1.txt" -F "sender=Петров" -F "work_id=hw1" http://localhost:8000/api/submit

echo -e "\n--- 3. Сидоров сдаёт другую работу (hw1) ---"
curl -F "file=@$TESTDIR/sidorov_hw1.txt" -F "sender=Сидоров" -F "work_id=hw1" http://localhost:8000/api/submit

echo -e "\n--- 4. Отчёт по hw1 (ожидание флагов плагиата) ---"
curl http://localhost:8000/api/works/hw1/reports | jq .

# cleanup
rm -rf $TESTDIR
