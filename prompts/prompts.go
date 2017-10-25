package prompts

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/manifoldco/go-manifold"
	"github.com/manifoldco/promptui"
	"github.com/rhymond/go-money"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/token"

	"github.com/manifoldco/manifold-cli/errs"
	"github.com/manifoldco/manifold-cli/prompts/templates"

	cModels "github.com/manifoldco/manifold-cli/generated/catalog/models"
	iModels "github.com/manifoldco/manifold-cli/generated/identity/models"
	mModels "github.com/manifoldco/manifold-cli/generated/marketplace/models"
)

const (
	namePattern   = "^[a-zA-Z\\s,\\.'\\-pL]{1,64}$"
	couponPattern = "^[0-9A-Z]{1,128}$"
	codePattern   = "^[0-9abcdefghjkmnpqrtuvwxyz]{16}$"
)

// NumberMask is the character used to mask number inputs
const NumberMask = '#'

var errBad = errors.New("Bad Value")

// SelectProduct prompts the user to select a product from the given list.
func SelectProduct(products []*cModels.Product, label string) (int, string, error) {
	var idx int
	if label != "" {
		found := false
		for i, p := range products {
			if string(p.Body.Label) == label {
				idx = i
				found = true
				break
			}
		}

		if !found {
			fmt.Println(promptui.FailedValue("Product", label))
			return 0, "", errs.ErrProductNotFound
		}

		fmt.Println(promptui.SuccessfulValue("Product", label)) // FIXME
		return idx, label, nil
	}

	sort.Slice(products, func(i, j int) bool {
		a := string(products[i].Body.Name)
		b := string(products[j].Body.Name)
		return strings.ToLower(a) < strings.ToLower(b)
	})

	prompt := promptui.Select{
		Label:     "Select Product",
		Items:     products,
		Templates: templates.TplProduct,
	}

	return prompt.Run()
}

// SelectPlan prompts the user to select a plan from the given list.
func SelectPlan(plans []*cModels.Plan, label string) (int, string, error) {
	var idx int
	if label != "" {
		found := false
		for i, p := range plans {
			if string(p.Body.Label) == label {
				idx = i
				found = true
				break
			}
		}

		if !found {
			fmt.Println(promptui.FailedValue("Plan", label))
			return 0, "", errs.ErrPlanNotFound
		}

		fmt.Println(promptui.SuccessfulValue("Plan", label)) //FIXME
		return idx, label, nil
	}

	sort.Slice(plans, func(i, j int) bool {
		a := plans[i]
		b := plans[j]

		if *a.Body.Cost == *b.Body.Cost {
			return strings.ToLower(string(a.Body.Name)) <
				strings.ToLower(string(b.Body.Name))
		}
		return *a.Body.Cost < *b.Body.Cost
	})

	prompt := promptui.Select{
		Label:     "Select Plan",
		Items:     plans,
		Templates: templates.TplPlan,
	}

	return prompt.Run()
}

// SelectResource promps the user to select a provisioned resource from the given list
func SelectResource(resources []*mModels.Resource, projects []*mModels.Project,
	label string) (int, string, error) {

	var idx int
	if label != "" {
		found := false
		for i, p := range resources {
			if string(p.Body.Label) == label {
				idx = i
				found = true
				break
			}
		}

		if !found {
			fmt.Println(promptui.FailedValue("Resource", label))
			return 0, "", errs.ErrResourceNotFound
		}

		fmt.Println(promptui.SuccessfulValue("Resource", label)) //FIXME
		return idx, label, nil
	}

	prompt := promptui.Select{
		Label:     "Select Resource",
		Items:     templates.Resources(resources, projects),
		Templates: templates.TplResource,
	}

	return prompt.Run()
}

// SelectRole prompts the user to select a role from the given list.
func SelectRole() (string, error) {
	prompt := promptui.Select{
		Label: "Select Role",
		Items: []string{"read", "read-credentials", "write", "admin"},
	}
	_, role, err := prompt.Run()
	return role, err
}

// SelectRegion prompts the user to select a region from the given list.
func SelectRegion(regions []*cModels.Region) (int, string, error) {
	line := func(r *cModels.Region) string {
		return fmt.Sprintf("%s (%s::%s)", r.Body.Name, *r.Body.Platform, *r.Body.Location)
	}

	labels := make([]string, len(regions))
	for i, r := range regions {
		labels[i] = line(r)
	}

	// TODO: Build "auto" resolve into promptui in case of only one item
	if len(regions) == 1 {
		fmt.Println(promptui.SuccessfulValue("Region", line(regions[0])))
		return 0, string(regions[0].Body.Name), nil
	}

	prompt := promptui.Select{
		Label: "Select Region",
		Items: labels,
	}

	return prompt.Run()
}

// SelectProject prompts the user to select a project from the given list.
func SelectProject(mProjects []*mModels.Project, label string, emptyOption bool) (int, string, error) {
	projects := make([]templates.Project, len(mProjects))
	for i, p := range mProjects {
		projects[i] = templates.Project{Name: p.Body.Label, Title: p.Body.Name}
	}

	var idx int
	if label != "" {
		found := false
		for i, p := range mProjects {
			if string(p.Body.Label) == label {
				idx = i
				found = true
				break
			}
		}

		if !found {
			fmt.Println(promptui.FailedValue("Select Project", label))
			return 0, "", errs.ErrProjectNotFound
		}

		fmt.Println(promptui.SuccessfulValue("Select Project", label)) //FIXME
		return idx, label, nil
	}

	if emptyOption {
		projects = append([]templates.Project{{Name: "No Project"}}, projects...)
	}

	prompt := promptui.Select{
		Label:     "Select Project",
		Items:     projects,
		Templates: templates.TplProject,
	}

	projectIdx, name, err := prompt.Run()

	if emptyOption {
		return projectIdx - 1, name, err
	}

	return projectIdx, name, err
}

// SelectContext runs a SelectTeam for context purposes
func SelectContext(teams []*iModels.Team, label string, userTuple *[]string) (int, string, error) {
	return selectTeam(teams, "Switch To", label, userTuple)
}

// SelectTeam prompts the user to select a team from the given list. -1 as the first return value
// indicates no team has been selected
func SelectTeam(teams []*iModels.Team, label string, userTuple *[]string) (int, string, error) {
	return selectTeam(teams, "Select Team", label, userTuple)
}

func selectTeam(mTeams []*iModels.Team, prefix, label string, userTuple *[]string) (int, string, error) {
	if prefix == "" {
		prefix = "Select Team"
	}

	var idx int
	if label != "" {
		found := false
		for i, t := range mTeams {
			if string(t.Body.Label) == label {
				idx = i
				found = true
				break
			}
		}

		if !found {
			fmt.Println(promptui.FailedValue("Team", label))
			return 0, "", errs.ErrTeamNotFound
		}

		fmt.Println(promptui.SuccessfulValue("Team", label)) // FIXME
		return idx, label, nil
	}

	teams := templates.Teams(mTeams)

	if userTuple != nil {
		u := *userTuple
		user := templates.Team{
			Name:  u[0],
			Title: u[1],
		}

		// treat user as a team for display purposes
		teams = append([]templates.Team{user}, teams...)
	}

	tpl := templates.TplTeam
	tpl.Selected = fmt.Sprintf(tpl.Selected, prefix)

	prompt := promptui.Select{
		Label:     prefix,
		Items:     teams,
		Templates: tpl,
	}

	teamIdx, name, err := prompt.Run()

	if userTuple != nil {
		return teamIdx - 1, name, err
	}

	return teamIdx, name, err
}

// ResourceTitle prompts the user to provide a resource title or to accept empty
// to let the system generate one.
func ResourceTitle(defaultValue string, autoSelect bool) (string, error) {
	validate := func(input string) error {
		if len(input) == 0 {
			return nil
		}

		t := manifold.Name(input)
		if err := t.Validate(nil); err != nil {
			return errors.New("Please provide a valid resource title")
		}

		return nil
	}

	label := "Resource Title (one will be generated if left blank)"

	if autoSelect {
		err := validate(defaultValue)
		if err != nil {
			fmt.Println(promptui.FailedValue(label, defaultValue))
		} else {
			fmt.Println(promptui.SuccessfulValue(label, defaultValue))
		}
		return defaultValue, err
	}

	p := promptui.Prompt{
		Label:    label,
		Default:  defaultValue,
		Validate: validate,
	}

	return p.Run()
}

// ResourceName prompts the user to provide a label name
func ResourceName(defaultValue string, autoSelect bool) (string, error) {
	validate := func(input string) error {
		if len(input) == 0 {
			return errors.New("Please provide a resource name")
		}

		l := manifold.Label(input)
		if err := l.Validate(nil); err != nil {
			return errors.New("Please provide a valid resource name")
		}

		return nil
	}

	label := "Resource Name"

	if autoSelect {
		err := validate(defaultValue)
		if err != nil {
			fmt.Println(promptui.FailedValue(label, defaultValue))
		} else {
			fmt.Println(promptui.SuccessfulValue(label, defaultValue))
		}

		return defaultValue, err
	}

	p := promptui.Prompt{
		Label:    label,
		Default:  defaultValue,
		Validate: validate,
	}

	return p.Run()
}

// TeamTitle prompts the user to enter a new Team title
func TeamTitle(defaultValue string, autoSelect bool) (string, error) {
	validate := func(input string) error {
		if len(input) == 0 {
			return errors.New("Please provide a valid team title")
		}

		l := manifold.Name(input)
		if err := l.Validate(nil); err != nil {
			return errors.New("Please provide a valid team title")
		}

		return nil
	}

	label := "Team Title"

	if autoSelect {
		err := validate(defaultValue)
		if err != nil {
			fmt.Println(promptui.FailedValue(label, defaultValue))
		} else {
			fmt.Println(promptui.SuccessfulValue(label, defaultValue))
		}
		return defaultValue, err
	}

	p := promptui.Prompt{
		Label:    label,
		Default:  defaultValue,
		Validate: validate,
	}

	return p.Run()
}

// ProjectTitle prompts the user to enter a new project title
func ProjectTitle(defaultValue string, autoSelect bool) (string, error) {
	validate := func(input string) error {
		if len(input) == 0 {
			return errors.New("Please provide a valid project title")
		}

		l := manifold.Name(input)
		if err := l.Validate(nil); err != nil {
			return errors.New("Please provide a valid project title")
		}

		return nil
	}

	label := "Project Title"

	if autoSelect {
		err := validate(defaultValue)
		if err != nil {
			fmt.Println(promptui.FailedValue(label, defaultValue))
		} else {
			fmt.Println(promptui.SuccessfulValue(label, defaultValue))
		}
		return defaultValue, err
	}

	p := promptui.Prompt{
		Label:    label,
		Default:  defaultValue,
		Validate: validate,
	}

	return p.Run()
}

// TokenDescription prompts the user to enter a token description
func TokenDescription() (string, error) {
	p := promptui.Prompt{
		Label:   "Token Description",
		Default: "",
	}
	return p.Run()
}

// ProjectDescription prompts the user to enter a project description
func ProjectDescription(defaultValue string, autoSelect bool) (string, error) {
	label := "Project Description"

	if autoSelect && defaultValue != "" {
		fmt.Println(promptui.SuccessfulValue(label, defaultValue))
		return defaultValue, nil
	}

	p := promptui.Prompt{
		Label:   label,
		Default: defaultValue,
	}

	return p.Run()
}

// Email prompts the user to provide an email *or* accepted the default
// email value
func Email(defaultValue string) (string, error) {
	p := promptui.Prompt{
		Label: "Email",
		Validate: func(input string) error {
			valid := govalidator.IsEmail(input)
			if valid {
				return nil
			}

			return errors.New("Please enter a valid email address")
		},
	}

	if defaultValue != "" {
		p.Default = defaultValue
	}

	return p.Run()
}

// FullName prompts the user to input a person's name
func FullName(defaultValue string) (string, error) {
	p := promptui.Prompt{
		Label: "Name",
		Validate: func(input string) error {
			if govalidator.StringMatches(input, namePattern) {
				return nil
			}
			return errors.New("Please enter a valid name")
		},
	}
	if defaultValue != "" {
		p.Default = defaultValue
	}

	return p.Run()
}

// CouponCode prompts the user to input an alphanumeric coupon code.
func CouponCode() (string, error) {
	p := promptui.Prompt{
		Label: "Code",
		Validate: func(input string) error {
			if govalidator.StringMatches(input, couponPattern) {
				return nil
			}
			return errors.New("Please enter a valid code")
		},
	}

	return p.Run()
}

// EmailVerificationCode prompts the user to input a person's name
func EmailVerificationCode(defaultValue string) (string, error) {
	p := promptui.Prompt{
		Label: "E-mail Verification Code",
		Validate: func(input string) error {
			if govalidator.StringMatches(input, codePattern) {
				return nil
			}
			return errors.New("Please enter a valid e-mail verification code")
		},
	}
	if defaultValue != "" {
		p.Default = defaultValue
	}

	return p.Run()
}

// PasswordMask is the character used to mask password inputs
const PasswordMask = '●'

// Password prompts the user to input a password value
func Password() (string, error) {
	prompt := promptui.Prompt{
		Label: "Password",
		Mask:  PasswordMask,
		Validate: func(input string) error {
			if len(input) < 8 {
				return errors.New("Passwords must be greater than 8 characters")
			}

			return nil
		},
	}

	return prompt.Run()
}

// Confirm is a confirmation prompt
func Confirm(msg string) (string, error) {
	p := promptui.Prompt{
		Label:     msg,
		IsConfirm: true,
	}

	return p.Run()
}

// HandleSelectError returns a cli error if the error is not an EOF or
// Interrupt
func HandleSelectError(err error, generic string) error {
	if err == promptui.ErrEOF || err == promptui.ErrInterrupt {
		return err
	}

	return errs.NewErrorExitError(generic, err)
}

func getPlanCost(p *cModels.Plan) string {
	if p.Body.Cost == nil {
		return "Free!"
	}

	c := *p.Body.Cost
	if c == 0 {
		return "Free!"
	}

	return money.New(c, "USD").Display()
}

func isCard(raw string) error {
	if govalidator.StringLength(raw, "12", "19") && govalidator.IsNumeric(raw) {
		return nil
	}

	return errBad
}

func isExpiry(raw string) error {
	if govalidator.StringLength(raw, "5", "5") {
		return nil
	}

	return errBad
}

func isCVV(raw string) error {
	if govalidator.StringLength(raw, "3", "4") && govalidator.IsNumeric(raw) {
		return nil
	}

	return errBad
}

// CreditCard handles receiving and tokenizing payment information
func CreditCard() (*stripe.Token, error) {
	rCrd, err := (&promptui.Prompt{
		Label:    "💳  Card Number",
		Validate: isCard,
	}).Run()
	if err != nil {
		return nil, err
	}

	rExp, err := (&promptui.Prompt{
		Label:    "📅  Expiry (MM/YY)",
		Validate: isExpiry,
	}).Run()
	if err != nil {
		return nil, err
	}

	rCVV, err := (&promptui.Prompt{
		Label:    "🔒  CVV",
		Mask:     NumberMask,
		Validate: isCVV,
	}).Run()
	if err != nil {
		return nil, err
	}

	parts := strings.Split(rExp, "/")
	year, month := "20"+parts[1], parts[0]
	tkn, err := token.New(&stripe.TokenParams{Card: &stripe.CardParams{
		Number: rCrd,
		Month:  month,
		Year:   year,
		CVC:    rCVV,
	}})
	if err != nil {
		return nil, errs.NewStripeError(err)
	}

	return tkn, nil
}

// SelectProvider prompts the user to select a provider resource from the given
// list.
func SelectProvider(mProviders []*cModels.Provider) (*cModels.Provider, error) {
	providers := templates.Providers(mProviders)

	label := templates.Provider{Name: "All Providers"}
	providers = append([]templates.Provider{label}, providers...)

	prompt := promptui.Select{
		Label:     "Select Provider",
		Items:     providers,
		Templates: templates.TplProvider,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	if idx == 0 {
		return nil, nil
	}

	return mProviders[idx-1], nil
}

// SelectAPIToken prompts the user to choose from a list of tokens
func SelectAPIToken(tokens []*iModels.APIToken) (*iModels.APIToken, error) {
	var labels []string
	for _, t := range tokens {
		val := fmt.Sprintf("%s****%s", *t.Body.FirstFour, *t.Body.LastFour)
		labels = append(labels, fmt.Sprintf("%s - %s", val, *t.Body.Description))
	}

	prompt := promptui.Select{
		Label: "Select API token",
		Items: labels,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	return tokens[idx], nil
}
