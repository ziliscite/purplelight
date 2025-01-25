package data

type Metadata struct {
	CurrentPage  int `json:"current_page,omitempty"`
	PageSize     int `json:"page_size,omitempty"`
	FirstPage    int `json:"first_page,omitempty"`
	LastPage     int `json:"last_page,omitempty"`
	TotalRecords int `json:"total_records,omitempty"`
}

// CalculateMetadata function calculates the appropriate pagination metadata
// values given the total number of records, current page, and page size values. Note
// that when the last page value is calculated, we are dividing two int values, and
// when dividing integer types in Go the result will also be an integer type, with
// the modulus (or remainder) dropped. So, for example, if there were 12 records in total
// and a page size of 5, the last page value would be (12+5-1)/5 = 3.2, which is then
// truncated to 3 by Go.
func (m *Metadata) CalculateMetadata(totalRecords, page, pageSize int) {
	if totalRecords == 0 {
		// Note that we return an empty Metadata struct if there are no records.
		return
	}

	m.CurrentPage = page
	m.PageSize = pageSize
	m.FirstPage = 1
	m.LastPage = (totalRecords + pageSize - 1) / pageSize
	m.TotalRecords = totalRecords
}
