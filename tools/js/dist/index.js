// Read input from stdin
function readInput() {
    const chunkSize = 1024;
    const inputChunks = [];
    let totalBytes = 0;
    // Read all the available bytes
    while (1) {
        const buffer = new Uint8Array(chunkSize);
        // Stdin file descriptor
        const fd = 0;
        // @ts-ignore
        const bytesRead = Javy.IO.readSync(fd, buffer);
        totalBytes += bytesRead;
        if (bytesRead === 0) {
            break;
        }
        inputChunks.push(buffer.subarray(0, bytesRead));
    }
    // Assemble input into a single Uint8Array
    const { finalBuffer } = inputChunks.reduce((context, chunk) => {
        context.finalBuffer.set(chunk, context.bufferOffset);
        context.bufferOffset += chunk.length;
        return context;
    }, { bufferOffset: 0, finalBuffer: new Uint8Array(totalBytes) });
    const maybeJson = new TextDecoder().decode(finalBuffer);
    try {
        return JSON.parse(maybeJson);
    }
    catch {
        return {};
    }
}
// Write output to stdout
function writeOutput(output) {
    const encodedOutput = new TextEncoder().encode(JSON.stringify(output));
    const buffer = new Uint8Array(encodedOutput);
    // Stdout file descriptor
    const fd = 1;
    // @ts-ignore
    Javy.IO.writeSync(fd, buffer);
}
export function handleWasm(handleRequest, handleResponse) {
    const input = {
        context: null,
        response: {
            body: null,
            headers: {},
        },
        request: {},
        error: null,
        ...readInput(),
    };
    let output = {
        request: {
            body: '',
            headers: {},
            method: '',
            url: '',
        },
        response: {
            body: '',
            headers: {},
            status: 0,
        },
        error: '',
    };
    switch (input.context) {
        case 'request':
            output = handleRequest(input);
            break;
        case 'response':
            output = handleResponse(input);
            break;
    }
    writeOutput(output);
}
//# sourceMappingURL=index.js.map