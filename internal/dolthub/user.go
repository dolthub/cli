package dolthub

import "context"

type EmailAddress struct {
	Address    string `json:"address"`
	IsPrimary  bool   `json:"is_primary"`
	IsVerified bool   `json:"is_verified"`
}
type User struct {
	Username       string         `json:"username"`
	DisplayName    string         `json:"display_name"`
	Bio            string         `json:"bio"`
	Location       string         `json:"location"`
	WebsiteURL     string         `json:"website_url"`
	ProfilePicURL  string         `json:"profile_pic_url"`
	EmailAddresses []EmailAddress `json:"email_addresses"`
}

// CurrentUser returns the authenticated DoltHub user.
func (c *Client) CurrentUser(ctx context.Context) (User, error) {
	var result envelope[User]
	err := c.get(ctx, "user", &result)
	return result.Data, err
}
