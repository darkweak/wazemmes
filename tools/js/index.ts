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
    const { finalBuffer } = inputChunks.reduce(
        (context, chunk) => {
            context.finalBuffer.set(chunk, context.bufferOffset);
            context.bufferOffset += chunk.length;
            return context;
        },
        { bufferOffset: 0, finalBuffer: new Uint8Array(totalBytes) },
    );

    const maybeJson = new TextDecoder().decode(finalBuffer);
    try {
        return JSON.parse(maybeJson);
    } catch {
        return {};
    }
}

// Write output to stdout
function writeOutput(output: Output) {
    const encodedOutput = new TextEncoder().encode(JSON.stringify(output));
    const buffer = new Uint8Array(encodedOutput);
    // Stdout file descriptor
    const fd = 1;
    // @ts-ignore
    Javy.IO.writeSync(fd, buffer);
}

type Context = 'request' | 'response';
type Request = {
    method: string;
    url: string;
    headers: Record<string, string>;
    body: string;
};
type Response = {
    status: number;
    headers: Record<string, string>;
    body: string;
}

type BaseHandler = {
    request: Request;
    response: Response;
    error: '';
}

type Input = BaseHandler & { context: Context };
type Output = BaseHandler;

type HandleRequest = (input: Input) => Output;
type HandleResponse = (input: Input) => Output;

export function handleWasm(handleRequest: HandleRequest, handleResponse: HandleResponse) {
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
    let output: Output = {
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
