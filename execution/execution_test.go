package execution

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type ExecutionTestSuite struct {
	suite.Suite
}

func TestExecutionTestSuite(t *testing.T) {
	suite.Run(t, new(ExecutionTestSuite))
}

type DummyMigration struct {
	version uint64
}

func (dm *DummyMigration) Version() uint64 {
	return dm.version
}

func (dm *DummyMigration) Up() error   { return nil }
func (dm *DummyMigration) Down() error { return nil }

func (suite *ExecutionTestSuite) TestItCanStartExecution() {
	mig := &DummyMigration{123}
	timeBefore := uint64(time.Now().UnixMilli())
	execution := StartExecution(mig)
	timeAfter := uint64(time.Now().UnixMilli())

	suite.Assert().Equal(mig.Version(), execution.Version)
	suite.Assert().True(
		execution.ExecutedAtMs >= timeBefore && execution.ExecutedAtMs <= timeAfter,
	)
	suite.Assert().Equal(uint64(0), execution.FinishedAtMs)
	suite.Assert().False(execution.Finished())
}

func (suite *ExecutionTestSuite) TestItCanFinishExecution() {
	execution := StartExecution(&DummyMigration{123})

	timeBefore := uint64(time.Now().UnixMilli())
	execution.FinishExecution()
	timeAfter := uint64(time.Now().UnixMilli())

	suite.Assert().True(
		execution.FinishedAtMs >= timeBefore && execution.FinishedAtMs <= timeAfter,
	)
	suite.Assert().True(execution.Finished())
}
