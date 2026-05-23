local base = {
  host: "localhost",
  port: 8080,
};

{
  prod: base { host: "prod.example.com" },
  staging: base { host: "staging.example.com" },
}
