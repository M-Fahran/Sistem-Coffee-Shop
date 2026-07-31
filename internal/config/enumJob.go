// enumJob.go — enum untuk antrean pekerjaan latar belakang.
//
// Memakai helper generik yang sama dengan enum.go (enumName, enumScan, dst),
// jadi perilakunya identik: SMALLINT di database, string di JSON, nilai 0
// tidak pernah valid.
package config

import "database/sql/driver"

// ============================================================================
// JobType
// ============================================================================

type JobType int16

const (
	// JobTypeInvoiceEmail mengirim struk setelah pembayaran lunas.
	JobTypeInvoiceEmail JobType = 1

	// JobTypeOrderReadyEmail memberi tahu pelanggan pesanannya siap diambil.
	JobTypeOrderReadyEmail JobType = 2
)

var (
	jobTypeNames = map[JobType]string{
		JobTypeInvoiceEmail:    "invoice_email",
		JobTypeOrderReadyEmail: "order_ready_email",
	}
	jobTypeValues = invertEnum(jobTypeNames)
)

func (t JobType) String() string { return enumName(t, jobTypeNames) }
func (t JobType) Valid() bool    { return enumValid(t, jobTypeNames) }

func (t JobType) MarshalJSON() ([]byte, error) {
	return enumMarshal(t, jobTypeNames, "job type")
}
func (t JobType) Value() (driver.Value, error) {
	return enumValue(t, jobTypeNames, "job type")
}
func (t *JobType) UnmarshalJSON(b []byte) error {
	return enumUnmarshal(b, t, jobTypeValues, "job type")
}
func (t *JobType) Scan(src any) error {
	return enumScan(src, t, jobTypeNames, "job type")
}

func ParseJobType(s string) (JobType, error) {
	return enumParse(s, jobTypeValues, "job type")
}

// ============================================================================
// JobStatus
// ============================================================================

type JobStatus int16

const (
	JobStatusPending    JobStatus = 1
	JobStatusProcessing JobStatus = 2
	JobStatusDone       JobStatus = 3
	JobStatusFailed     JobStatus = 4
)

var (
	jobStatusNames = map[JobStatus]string{
		JobStatusPending:    "pending",
		JobStatusProcessing: "processing",
		JobStatusDone:       "done",
		JobStatusFailed:     "failed",
	}
	jobStatusValues = invertEnum(jobStatusNames)
)

func (s JobStatus) String() string { return enumName(s, jobStatusNames) }
func (s JobStatus) Valid() bool    { return enumValid(s, jobStatusNames) }

func (s JobStatus) MarshalJSON() ([]byte, error) {
	return enumMarshal(s, jobStatusNames, "job status")
}
func (s JobStatus) Value() (driver.Value, error) {
	return enumValue(s, jobStatusNames, "job status")
}
func (s *JobStatus) UnmarshalJSON(b []byte) error {
	return enumUnmarshal(b, s, jobStatusValues, "job status")
}
func (s *JobStatus) Scan(src any) error {
	return enumScan(src, s, jobStatusNames, "job status")
}

func ParseJobStatus(str string) (JobStatus, error) {
	return enumParse(str, jobStatusValues, "job status")
}