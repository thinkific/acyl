# testhelper/localdb

A package to run a local database for tests

Usage:

```go
func TestMain(m *testing.M) {
	var exit int
	defer func() { os.Exit(exit) }() // exit with proper code depending on test results
	ldb := localdb.New(log.Printf)
	ldb.MustRun()
	defer ldb.Stop()
	exit = m.Run()
}
```

# testhelper/testdatalayer

A package which utilizes a local database service, populates it with test data and creates an instance of DataLayer using it suitable for use in tests

Usage:

```go
func TestFoo(t *testing.T) {
	dl, tdl := testdatalayer.New(testlogger, t)
	if err := tdl.Setup(); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
  // ...
  // dl.GetQAEnvironment(), etc
}
```
