(() => {
  // node_modules/wazemmes/dist/index.js
  function readInput() {
    const chunkSize = 1024;
    const inputChunks = [];
    let totalBytes = 0;
    while (1) {
      const buffer = new Uint8Array(chunkSize);
      const fd = 0;
      const bytesRead = Javy.IO.readSync(fd, buffer);
      totalBytes += bytesRead;
      if (bytesRead === 0) {
        break;
      }
      inputChunks.push(buffer.subarray(0, bytesRead));
    }
    const { finalBuffer } = inputChunks.reduce((context, chunk) => {
      context.finalBuffer.set(chunk, context.bufferOffset);
      context.bufferOffset += chunk.length;
      return context;
    }, { bufferOffset: 0, finalBuffer: new Uint8Array(totalBytes) });
    const maybeJson = new TextDecoder().decode(finalBuffer);
    try {
      return JSON.parse(maybeJson);
    } catch {
      return {};
    }
  }
  function writeOutput(output) {
    const encodedOutput = new TextEncoder().encode(JSON.stringify(output));
    const buffer = new Uint8Array(encodedOutput);
    const fd = 1;
    Javy.IO.writeSync(fd, buffer);
  }
  function handleWasm(handleRequest2, handleResponse2) {
    const input = {
      context: null,
      response: {
        body: null,
        headers: {}
      },
      request: {},
      error: null,
      ...readInput()
    };
    let output = {
      request: {
        body: "",
        headers: {},
        method: "",
        url: ""
      },
      response: {
        body: "",
        headers: {},
        status: 0
      },
      error: ""
    };
    switch (input.context) {
      case "request":
        output = handleRequest2(input);
        break;
      case "response":
        output = handleResponse2(input);
        break;
    }
    writeOutput(output);
  }

  // handler.js
  function handleRequest(input) {
    input.response.body = input.request.body;
    input.response.headers["X-Plugin-Request"] = ["DONE"];
    input.error = "should stop";
    return input;
  }
  function handleResponse(input) {
    input.response.headers["X-Plugin-Response"] = ["DONE"];
    input.error = "should stop";
    return input;
  }
  handleWasm(handleRequest, handleResponse);
})();
