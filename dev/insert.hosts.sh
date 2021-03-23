#!/bin/bash

NUM_HOSTS=$1

if [ "$NUM_HOSTS" -gt 10000 ]; then
  echo "Maximum number of hosts is 10000"
  exit 1
fi

FILE=/tmp/insert.hosts.$(uuidgen).sql

trap ctrl_c INT

function ctrl_c() {
  rm "$FILE"
  exit 0
}

SQL_STATEMENT="INSERT INTO hosts
  (id, account, display_name, created_on, modified_on, facts, tags, canonical_facts, system_profile_facts, stale_timestamp, reporter)
  VALUES "

echo "$SQL_STATEMENT" > "$FILE"

for ((i = 1 ; i <= NUM_HOSTS ; i++)); do
  VALUE="(
  '$(uuidgen)',
  '5',
  'test',
  '2017-01-01 00:00:00',
  '2017-01-01 00:00:00',
  '{\"test1\": \"test1a\"}',
  '{\"test2\": \"test2a\"}',
  '{\"test3\": \"test3a\", \"test4\": [\"asdf\", \"jkl;\"], \"test5\": {\"a\": {\"1\": \"123\"}}, \"test6\": [{\"1\": \"23\"}, {\"2\": {\"3\": \"345\"}}]}',
  '{}',
  '2017-01-01 00:00:00', 'me'
  )"

  echo "$VALUE" >> "$FILE"

  if [ "$i" != $((NUM_HOSTS)) ]; then
    echo ", " >> "$FILE"
  fi

  echo -ne "$i\r"
done

psql -U postgres -h localhost -p 5432 -d insights -f "$FILE"
rm "$FILE"