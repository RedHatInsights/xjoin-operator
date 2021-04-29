#!/bin/bash

NUM_HOSTS=$1
MODIFIED_ON_AGE=$2 #time in seconds to subtract from NOW for modified on field

CHUNK_SIZE=5000
NUM_CHUNKS=$((NUM_HOSTS/CHUNK_SIZE))
EXTRA_HOSTS=$((NUM_HOSTS%CHUNK_SIZE))

trap ctrl_c INT

NOW=$(date --date="-$MODIFIED_ON_AGE seconds" --utc "+%F %T")

function ctrl_c() {
  rm /tmp/insert.hosts.*.sql
  exit 0
}

function add_header() {
  FILE=$1
  SQL_STATEMENT="INSERT INTO hosts
    (id, account, display_name, created_on, modified_on, facts, tags, canonical_facts, system_profile_facts, stale_timestamp, reporter)
    VALUES "
  echo "$SQL_STATEMENT" > "$FILE"
}

function add_host() {
  FILE=$1

  VALUE="(
  '$(uuidgen)',
  '5',
  'test',
  '2017-01-01 00:00:00',
  '$NOW',
  '{\"test1\": \"test1a\"}',
  '{\"NS1\": {\"key3\": [\"val3\"]}, \"NS3\": {\"key3\": [\"val3\"]}, \"Sat\": {\"prod\": []}, \"SPECIAL\": {\"key\": [\"val\"]}}',
  '{\"fqdn\": \"ad7125.foo.redhat.com\", \"bios_uuid\": \"5308151c-3d6f-46a0-87c8-36ca4a03e148\"}',
  '{\"arch\": \"x86-64\", \"owner_id\": \"1b36b20f-7fa0-4454-a6d2-008294e06378\", \"cpu_flags\": [\"flag1\", \"flag2\"], \"cpu_model\": \"Intel(R) Xeon(R) CPU E5-2690 0 @ 2.90GHz\", \"yum_repos\": [{\"name\": \"repo1\", \"enabled\": true, \"base_ur    l\": \"http://rpms.redhat.com\", \"gpgcheck\": true}], \"os_release\": \"Red Hat EL 7.0.1\", \"bios_vendor\": \"Turd Ferguson\", \"bios_version\": \"1.0.0uhoh\", \"disk_devices\": [{\"type\": \"ext3\", \"label\": \"home drive\", \"device\": \"/dev/sdb1\", \"options\": {\"ro\": true, \"uid\": \"0\"}, \"mount_point\": \"/h    ome\"}], \"captured_date\": \"2020-02-13T12:16:00Z\", \"rhc_client_id\": \"044e36dc-4e2b-4e69-8948-9c65a7bf4976\", \"is_marketplace\": false, \"kernel_modules\": [\"i915\", \"e1000e\"], \"last_boot_time\": \"12:25 Mar 19, 2019\", \"number_of_cpus\": 1, \"cores_per_socket\": 4, \"enabled_services\": [\"ndb\",     \"krb5\"], \"rhc_config_state\": \"044e36dc-4e2b-4e69-8948-9c65a7bf4976\", \"bios_release_date\": \"10/31/2013\", \"number_of_sockets\": 2, \"os_kernel_version\": \"3.10.0\", \"running_processes\": [\"vim\", \"gcc\", \"python\"], \"satellite_managed\": false, \"installed_products\": [{\"id\": \"123\", \"name\":     \"eap\", \"status\": \"UP\"}, {\"id\": \"321\", \"name\": \"jbws\", \"status\": \"DOWN\"}], \"installed_services\": [\"ndb\", \"krb5\"], \"network_interfaces\": [{\"mtu\": 1500, \"name\": \"eth0\", \"type\": \"loopback\", \"state\": \"UP\", \"mac_address\": \"aa:bb:cc:dd:ee:ff\", \"ipv4_addresses\": [\"10.10.10.1\"], \"ipv6_add    resses\": [\"2001:0db8:85a3:0000:0000:8a2e:0370:7334\"]}], \"infrastructure_type\": \"jingleheimer junction cpu\", \"selinux_config_file\": \"enforcing\", \"subscription_status\": \"valid\", \"system_memory_bytes\": 1024, \"insights_egg_version\": \"120.0.1\", \"selinux_current_mode\": \"enforcing\", \"in    frastructure_vendor\": \"dell\", \"katello_agent_running\": false, \"insights_client_version\": \"12.0.12\", \"subscription_auto_attach\": \"yes\"}',
  '2017-01-01 00:00:00',
  'me'
  )"

  echo "$VALUE" >> "$FILE"
}

function insert_chunk() {
  FILE=/tmp/insert.hosts.$(uuidgen).sql
  add_header "$FILE"

  for ((i = 1 ; i <= CHUNK_SIZE ; i++)); do
    add_host "$FILE"

    if [ "$i" != $((CHUNK_SIZE)) ]; then
      echo ", " >> "$FILE"
    fi

    INDEX=$((i+j*CHUNK_SIZE))

    echo -ne "$INDEX\r"
  done

  psql -U postgres -h localhost -p 5432 -d insights -f "$FILE"
  rm "$FILE"
}

function insert_extras() {
  FILE=/tmp/insert.hosts.$(uuidgen).sql
  add_header "$FILE"
  for ((k = 1 ; k <= EXTRA_HOSTS ; k++)); do
    add_host "$FILE"
    if [ "$k" -lt "$EXTRA_HOSTS" ]; then
      echo ", " >> "$FILE"
    fi
    INDEX=$((INDEX+1))
    echo -ne "$INDEX\r"
  done

  psql -U postgres -h localhost -p 5432 -d insights -f "$FILE"
  rm "$FILE"
}

INDEX=0
for ((j = 0 ; j < NUM_CHUNKS ; j++)); do
  insert_chunk
done

if [[ EXTRA_HOSTS -gt 0 ]]; then
  insert_extras
fi

wait
