package data

type Host struct {
	ID                 *string             `json:"id" db:"id"`
	Account            *string             `json:"account" db:"account"`
	DisplayName        *string             `json:"display_name" db:"display_name"`
	CreatedOn          *string             `json:"created_on" db:"created_on"`
	ModifiedOn         *string             `json:"modified_on" db:"modified_on"`
	Facts              interface{}         `json:"facts" db:"facts"`
	CanonicalFacts     interface{}         `json:"canonical_facts" db:"canonical_facts"`
	SystemProfileFacts interface{}         `json:"system_profile_facts" db:"system_profile_facts"`
	AnsibleHost        *string             `json:"ansible_host" db:"ansible_host"`
	StaleTimestamp     *string             `json:"stale_timestamp" db:"stale_timestamp"`
	Reporter           *string             `json:"reporter" db:"reporter"`
	Tags               interface{}         `json:"tags" db:"tags"`
	TagsStructured     []map[string]string `json:"tags_structured"`
	TagsString         []string            `json:"tags_string"`
	TagsSearch         []string            `json:"tags_search"`
}
