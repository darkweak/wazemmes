<?php
header('Content-Type: application/json');

$method = $_SERVER['REQUEST_METHOD'] ?? 'GET';
$uri = $_SERVER['REQUEST_URI'] ?? '/';
$headers = getallheaders();

$response = [
    'request' => null,
    'response' => [
        'headers' => [],
        'body' => 'Hello from PHP WASI Middleware',
        'status' => 200,
    ],
    'error' => ''
];

echo json_encode($response, JSON_PRETTY_PRINT);
?>
