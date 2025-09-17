import { handleWasm } from 'wazemmes';

function handleRequest(input) {
    input.body = "Hello there!"

    return input;
}

function handleResponse(input) {
    input.body = "Hello there!"

    return input;
}

handleWasm(handleRequest, handleResponse);
