$name = $args[0]
if ([string]::IsNullOrWhiteSpace($name)) {
  echo "No name provided"
  return
}
wget http://localhost:8080/debug/pprof/profile?seconds=20 -o profiles/$name
go tool pprof -http=: profiles/$name