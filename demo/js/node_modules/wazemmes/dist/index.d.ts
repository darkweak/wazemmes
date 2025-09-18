type Context = 'request' | 'response';
type Request = {
    method: string;
    url: string;
    headers: Record<string, string[]>;
    body: string;
};
type Response = {
    status: number;
    headers: Record<string, string[]>;
    body: string;
};
type BaseHandler = {
    request: Request;
    response: Response;
    error: '';
};
type Input = BaseHandler & {
    context: Context;
};
type Output = BaseHandler;
type HandleRequest = (input: Input) => Output;
type HandleResponse = (input: Input) => Output;
export declare function handleWasm(handleRequest: HandleRequest, handleResponse: HandleResponse): void;
export {};
//# sourceMappingURL=index.d.ts.map