$name = $args[0]
if ([string]::IsNullOrWhiteSpace($name)) {
  echo "No name provided"
  return
}
# wget http://localhost:8080/debug/pprof/heap -o profiles/$name
wget http://localhost:8080/debug/pprof/trace?seconds=3 -o profiles/$name
go tool pprof -http=: profiles/$name
