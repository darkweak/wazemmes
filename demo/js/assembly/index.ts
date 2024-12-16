// The entry file of your WebAssembly module.

// @external("log", "integer")
// declare function logInteger(i: i32): void // { "log": { "integer"(i) { ... } } }

export function handle_request(): i64 {
  console.log("Hello demo");

  return 0;
}

export function handle_response(a: i32, b: i32): void {
  return;
}
