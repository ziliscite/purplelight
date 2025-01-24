package data

type Enumify interface {
	ToEnum(string) (string, error)
}
