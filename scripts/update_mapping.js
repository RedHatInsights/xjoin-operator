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


function getESIndexTemplate(parameters) {
    let ESIndexTemplate = null;

    parameters.forEach(parameter => {
        if (parameter.name == "ELASTICSEARCH_INDEX_TEMPLATE") {
            ESIndexTemplate = parameter.value;
        }
    });

    return JSON.parse(ESIndexTemplate);
}

function setESIndexTemplate(parameters, new_template) {
    new_template = JSON.stringify(new_template, null, 2);
    let i = 0;

    parameters.forEach(parameter => {
        if (parameter.name == "ELASTICSEARCH_INDEX_TEMPLATE") {
            parameters[i] = new_template;
        }
        i++;
    });
}

try {
    var myArgs = process.argv.slice(2);
    schemaPath = myArgs[0];

    console.log(schemaPath)

    if (typeof(schemaPath) != String) {
        schemaPath = './inventory-schemas/schemas/system_profile/v1.yaml'
    }

    let schemaFileContent = fs.readFileSync(schemaPath, 'utf8');
    let schemaData = yaml.load(schemaFileContent);
    let schema = schemaData["$defs"]["SystemProfile"]

    let deploymentFilePath = './deploy/operator.yml'
    let deploymentFileContent = fs.readFileSync(deploymentFilePath, 'utf8');
    let deploymentFileData = yaml.load(deploymentFileContent);

    let ESIndexTemplate = getESIndexTemplate(deploymentFileData["parameters"])

    schema = remove_blocked_fields(schema);
    new_mapping = buildMappingsFor("system_profile_facts", schema);

    console.log("new_mapping")
    console.log(new_mapping["mappings"]["system_profile_facts"])

    console.log("es template")
    console.log(ESIndexTemplate)

    ESIndexTemplate["mappings"]["properties"]["system_profile_facts"]["properties"] = new_mapping["mappings"]["system_profile_facts"]["properties"];
    setESIndexTemplate(deploymentFileData["parameters"], ESIndexTemplate)
    
    fs.writeFileSync(deploymentFilePath, yaml.dump(deploymentFileData,{"quotingType": "d"} ));

} catch (e) {
    console.log(e);
}