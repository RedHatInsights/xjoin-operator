const fs = require('fs');
const yaml = require('js-yaml');
const { buildMappingsFor } = require("json-schema-to-es-mapping");

const typeMap = {
    "string": "keyword",
    "integer": "keyword",
    "boolean": "boolean",
    "object": "object"
}

// function getEsType(key, value) {
//     console.log(key);
//     fieldType = value["type"];
//     indexType = typeMap[fieldType];
//     if (fieldType == "array") {
//         console.log("array");
//         console.log(key);
//         console.log(value);
//         value = value["items"];
//         indexType = getEsType(key, value);
//     }
//     if (fieldType == "object") {
//         //do something else
//         properties = {};
//         for (const [key, value] of Object.entries(value["properties"])) {
            
//         }
//         return {
//             "type": indexType,
//             "properties": properties
//         };
//     }

//     return { "type": indexType };
// }

try {
    var myArgs = process.argv.slice(2);
    schemaPath = myArgs[0];

    console.log(schemaPath)

    if (schemaPath == undefined) {
        schemaPath = './inventory-schemas/schemas/system_profile/v1.yaml'
    }

    let schemaFileContent = fs.readFileSync(myArgs[0], 'utf8');
    let schemaData = yaml.safeLoad(schemaFileContent);
    let schema = schemaData["$defs"]["SystemProfile"]

    let mappingFilePath = '../deploy/operator.yml'
    let mappingFileContent = fs.readFileSync(mappingFilePath, 'utf8');
    let mappingData = yaml.safeLoad(mappingFileContent);

    let template = JSON.parse(mappingData["objects"][3]["data"]["elasticsearch.index.template"]);

    
    new_mapping = buildMappingsFor("system_profile_facts", schema);

    console.log("new_mapping")
    console.log(new_mapping["mappings"]["system_profile_facts"])

    template["mappings"]["properties"]["system_profile_facts"]["properties"] = new_mapping["mappings"]["system_profile_facts"]["properties"];
    mappingData["objects"][3]["data"]["elasticsearch.index.template"] = JSON.stringify(template, null, 4);
    
    fs.writeFileSync(mappingFilePath, yaml.dump(mappingData,{"quotingType": ""} ));

} catch (e) {
    console.log(e);
}