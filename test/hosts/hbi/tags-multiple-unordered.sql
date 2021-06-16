INSERT INTO hosts (
    id,
    account,
    display_name,
    created_on,
    modified_on,
    facts,
    tags,
    canonical_facts,
    system_profile_facts,
    stale_timestamp,
    reporter)
VALUES (
    'bfd6dc3b-ea5a-4734-b3db-e311c728bb91',
    '5',
    'test',
    '2017-01-01 00:00:00',
    '2021-06-16 03:00:14+00',
    '{"test1": "test1a"}',
    '{"Sat": {"prod": ["val"]}, "ANS": {"key1": ["val1"], "akey2": ["val2"]}, "ZNS": {"key1": ["val3", "val1"]}}',
    '{"test3": "test3a", "test4": ["asdf", "jkl;"], "test5": {"a": {"1": "123"}}, "test6": [{"1": "23"}, {"2": {"3": "345"}}]}',
    '{"arch": "x86-64", "cpu_flags": ["flag1", "flag2"], "yum_repos": [{"name": "repo1", "enabled": true, "base_ur    l": "http://rpms.redhat.com", "gpgcheck": true}], "is_marketplace": false, "number_of_cpus": 1,  "number_of_sockets": 2, "anobject": {"key1": "value1", "key2": "value2", "array1": ["value1", "value2"]}}',
    '2017-01-01 00:00:00',
    'me')