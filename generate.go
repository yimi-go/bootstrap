package bootstrap

//go:generate mockgen --destination=runner_mock_test.go --package=bootstrap github.com/yimi-go/runner Runner
//go:generate mockgen --destination=shutdown_mock_test.go --package=bootstrap github.com/yimi-go/shutdown Trigger,Controller,Callback,Event,ErrorHandler
