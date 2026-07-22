package entity

type Table struct {
	ID 	  int64     `json:"id"`
	Number string   `json:"number"`
	QRToken string   `json:"qr_token"`
	IsActive bool     `json:"is_active"`
}


type CreateTableInput struct {
    Number   string
    IsActive bool
}

type UpdateTableInput struct {
    Number   string
    IsActive bool
}

type ListTablesInput struct {
    Search   string
    IsActive *bool
    Page     int
    PerPage  int
}

type ListResult struct {
    Items   []Table
    Total   int
    Page    int
    PerPage int
}

