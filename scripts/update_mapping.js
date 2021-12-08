// When running this script add the path to the schema file to use as argv
// e.g. node update_mapping.js path/to/file.yml

const fs = require('fs');
const yaml = require('js-yaml');
const { buildMappingsFor } = require("json-schema-to-es-mapping");

function remove_blocked_fields(schema) {
    for (const [key, value] of Object.entries(schema["properties"])) {
        console.log(key)
        console.log(value["type"]);

        console.log(typeof(value));

        if ("x-indexed" in value && value["x-indexed"] == false) {
            console.log("found x-indexed: " + value["x-indexed"]);
            delete schema["properties"][key]; 
        }
    }

    return schema;
}


try {
    var myArgs = process.argv.slice(2);
    schemaPath = myArgs[0];

    console.log(schemaPath)

    if (typeof(schemaPath) != String) {
        schemaPath = './inventory-schemas/schemas/system_profile/v1.yaml'
    }

    let schemaFileContent = fs.readFileSync(schemaPath, 'utf8');
    let schemaData = yaml.safeLoad(schemaFileContent);
    let schema = schemaData["$defs"]["SystemProfile"]

    let mappingFilePath = '../deploy/operator.yml'
    let mappingFileContent = fs.readFileSync(mappingFilePath, 'utf8');
    let mappingData = yaml.safeLoad(mappingFileContent);

    let template = JSON.parse(mappingData["objects"][3]["data"]["elasticsearch.index.template"]);

    schema = remove_blocked_fields(schema);
    new_mapping = buildMappingsFor("system_profile_facts", schema);

    console.log("new_mapping")
    console.log(new_mapping["mappings"]["system_profile_facts"])

    template["mappings"]["properties"]["system_profile_facts"]["properties"] = new_mapping["mappings"]["system_profile_facts"]["properties"];
    mappingData["objects"][3]["data"]["elasticsearch.index.template"] = JSON.stringify(template, null, 4);
    
    fs.writeFileSync(mappingFilePath, yaml.dump(mappingData,{"quotingType": "d"} ));

} catch (e) {
    console.log(e);
}