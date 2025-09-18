import { handleWasm } from 'wazemmes';

function handleRequest(input) {
    input.response.body = input.request.body;
    input.response.headers["X-Plugin-Request"] = ["DONE"];

    return input;
}

function handleResponse(input) {
    input.response.headers["X-Plugin-Response"] = ["DONE"];

    return input;
}

handleWasm(handleRequest, handleResponse);
