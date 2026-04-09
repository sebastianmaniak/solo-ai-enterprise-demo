// generate-customers generates 100 Solo Bank customer markdown profiles.
// Run from wiki-server directory: go run cmd/generate-customers/main.go
package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Constants and rate tables
// ---------------------------------------------------------------------------

// Mortgage rates by tier and type (CRITICAL — must match wiki rate tables)
var mortgageRates = map[int]map[string]float64{
	1: {"30yr Fixed": 6.125, "15yr Fixed": 5.375, "5/1 ARM": 5.750},
	2: {"30yr Fixed": 6.375, "15yr Fixed": 5.625, "5/1 ARM": 6.000},
	3: {"30yr Fixed": 6.875, "15yr Fixed": 6.125, "5/1 ARM": 6.500},
	4: {"30yr Fixed": 7.500, "15yr Fixed": 6.750},
}

// Credit card APRs by product and tier
var ccAPRs = map[string]map[int]float64{
	"Platinum Rewards Card": {1: 18.99, 2: 20.99},
	"Cashback Card":         {1: 19.99, 2: 21.99, 3: 24.99},
	"Secured Card":          {4: 22.99, 5: 22.99},
}

// Credit limit formulae: percentage of gross annual salary
var creditLimitPct = map[int]float64{
	1: 0.50,
	2: 0.40,
	3: 0.30,
	4: 0.20,
}

// Savings APY by tier
var savingsAPY = map[int]float64{
	1: 4.75,
	2: 4.50,
	3: 4.25,
	4: 3.75,
	5: 3.50,
}

// ---------------------------------------------------------------------------
// Name pools (100 unique first-last combos)
// ---------------------------------------------------------------------------

var firstNames = []string{
	"James", "Mary", "John", "Patricia", "Robert", "Jennifer", "Michael", "Linda",
	"William", "Barbara", "David", "Elizabeth", "Richard", "Susan", "Joseph", "Jessica",
	"Thomas", "Sarah", "Charles", "Karen", "Christopher", "Lisa", "Daniel", "Nancy",
	"Matthew", "Betty", "Anthony", "Margaret", "Mark", "Sandra", "Donald", "Ashley",
	"Steven", "Dorothy", "Paul", "Kimberly", "Andrew", "Emily", "Joshua", "Donna",
	"Kenneth", "Michelle", "Kevin", "Carol", "Brian", "Amanda", "George", "Melissa",
	"Timothy", "Deborah", "Ronald", "Stephanie", "Edward", "Rebecca", "Jason", "Sharon",
	"Jeffrey", "Laura", "Ryan", "Cynthia", "Jacob", "Kathleen", "Gary", "Amy",
	"Nicholas", "Angela", "Eric", "Shirley", "Jonathan", "Anna", "Stephen", "Brenda",
	"Larry", "Pamela", "Justin", "Emma", "Scott", "Nicole", "Brandon", "Helen",
	"Benjamin", "Samantha", "Samuel", "Katherine", "Raymond", "Christine", "Gregory", "Debra",
	"Frank", "Rachel", "Alexander", "Carolyn", "Patrick", "Janet", "Jack", "Catherine",
	"Dennis", "Maria", "Jerry", "Heather",
}

var lastNames = []string{
	"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis",
	"Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson", "Thomas",
	"Taylor", "Moore", "Jackson", "Martin", "Lee", "Perez", "Thompson", "White",
	"Harris", "Sanchez", "Clark", "Ramirez", "Lewis", "Robinson", "Walker", "Young",
	"Allen", "King", "Wright", "Scott", "Torres", "Nguyen", "Hill", "Flores",
	"Green", "Adams", "Nelson", "Baker", "Hall", "Rivera", "Campbell", "Mitchell",
	"Carter", "Roberts", "Gomez", "Phillips", "Evans", "Turner", "Diaz", "Parker",
	"Cruz", "Edwards", "Collins", "Reyes", "Stewart", "Morris", "Morales", "Murphy",
	"Cook", "Rogers", "Gutierrez", "Ortiz", "Morgan", "Cooper", "Peterson", "Bailey",
	"Reed", "Kelly", "Howard", "Ramos", "Kim", "Cox", "Ward", "Richardson",
	"Watson", "Brooks", "Chavez", "Wood", "James", "Bennett", "Gray", "Mendoza",
	"Ruiz", "Hughes", "Price", "Alvarez", "Castillo", "Sanders", "Patel", "Myers",
	"Long", "Ross", "Foster", "Jimenez",
}

// ---------------------------------------------------------------------------
// Merchant / transaction data
// ---------------------------------------------------------------------------

var merchants = []string{
	"Whole Foods Market", "Starbucks", "Amazon.com", "Target", "Walmart",
	"Home Depot", "Costco", "CVS Pharmacy", "Shell Gas Station", "Netflix",
	"Spotify", "Apple Store", "Best Buy", "TJ Maxx", "Publix Supermarket",
	"Chipotle Mexican Grill", "McDonald's", "Uber", "Lyft", "Southwest Airlines",
	"Delta Airlines", "Marriott Hotels", "Hilton Hotels", "Cheesecake Factory",
	"Olive Garden", "PetSmart", "Dick's Sporting Goods", "Gap", "Macy's",
	"Nordstrom", "Lowe's", "IKEA", "Trader Joe's", "Kroger", "Walgreens",
}

var cities = []string{
	"Springfield, IL", "Columbus, OH", "Nashville, TN", "Charlotte, NC",
	"Indianapolis, IN", "Austin, TX", "Jacksonville, FL", "Denver, CO",
	"Memphis, TN", "Louisville, KY", "Baltimore, MD", "Milwaukee, WI",
	"Albuquerque, NM", "Tucson, AZ", "Fresno, CA", "Sacramento, CA",
	"Mesa, AZ", "Kansas City, MO", "Atlanta, GA", "Omaha, NE",
	"Colorado Springs, CO", "Raleigh, NC", "Long Beach, CA", "Virginia Beach, VA",
}

var streetNames = []string{
	"Oak Lane", "Maple Street", "Cedar Drive", "Pine Avenue", "Elm Court",
	"Birch Road", "Walnut Boulevard", "Cherry Hill Drive", "Spruce Street",
	"Willow Way", "Hickory Lane", "Sycamore Avenue", "Poplar Court",
	"Magnolia Drive", "Chestnut Street", "Ash Boulevard",
}

var employers = []string{
	"TechCorp Inc", "Midwest Regional Hospital", "First National Insurance", "BuildRight Construction",
	"Superior Logistics", "Green Valley Schools", "City of Springfield", "Apex Financial Group",
	"DataSolutions LLC", "Summit Healthcare", "Riverfront Manufacturing", "Allied Retail Group",
	"NovaTech Systems", "Clearwater Law Firm", "BlueSky Airlines", "Precision Engineering",
	"Metro Transit Authority", "Pinnacle Consulting", "Lakeside Medical Center", "Heritage Real Estate",
	"QuickShip Delivery", "Golden State University", "Pacific Trade Co", "United Services Corp",
	"Keystone Energy", "Bright Future Education", "Harbor Freight Solutions", "Central Park Hotel",
	"Diamond Software", "Iron Gate Security",
}

var jobTitles = []string{
	"Senior Software Engineer", "Registered Nurse", "Insurance Adjuster", "Project Manager",
	"Logistics Coordinator", "High School Teacher", "Administrative Analyst", "Financial Advisor",
	"Data Scientist", "Physical Therapist", "Plant Supervisor", "Retail Manager",
	"Systems Architect", "Associate Attorney", "Flight Attendant", "Mechanical Engineer",
	"Transit Dispatcher", "Management Consultant", "Radiologist", "Real Estate Agent",
	"Delivery Driver", "Professor", "Import Specialist", "Customer Service Manager",
	"Petroleum Engineer", "Curriculum Developer", "Warehouse Manager", "Hotel Manager",
	"Software Developer", "Security Analyst",
}

// ---------------------------------------------------------------------------
// Data structures
// ---------------------------------------------------------------------------

type Account struct {
	ID      string
	Type    string
	Balance float64
	APY     float64 // for savings
}

type CreditCard struct {
	ID      string
	Product string
	Limit   int
	Balance float64
	APR     float64
	History string
}

type Mortgage struct {
	Type          string
	Rate          float64
	Principal     float64
	Property      string
	MonthlyPmt    float64
	RemainingTerm int // years
}

type Transaction struct {
	Date        string
	Description string
	Amount      float64
	Account     string
}

type Customer struct {
	Num          int
	ID           string
	FirstName    string
	LastName     string
	Age          int
	DOB          string
	Employer     string
	JobTitle     string
	Salary       int
	CreditScore  int
	Tier         int
	TierLabel    string
	RiskRating   string
	CustomerSince string
	Accounts     []Account
	Cards        []CreditCard
	Mortgage     *Mortgage
	Transactions []Transaction
	Notes        []string
}

// ---------------------------------------------------------------------------
// Tier helpers
// ---------------------------------------------------------------------------

func scoreTier(score int) (int, string) {
	switch {
	case score >= 800:
		return 1, "Excellent"
	case score >= 740:
		return 2, "Very Good"
	case score >= 670:
		return 3, "Good"
	case score >= 580:
		return 4, "Fair"
	default:
		return 5, "Poor"
	}
}

func tierRisk(tier int) string {
	switch tier {
	case 1:
		return "Very Low"
	case 2:
		return "Low"
	case 3:
		return "Moderate"
	case 4:
		return "High"
	default:
		return "Very High"
	}
}

// ---------------------------------------------------------------------------
// Financial calculation helpers
// ---------------------------------------------------------------------------

// monthlyPayment calculates standard amortization monthly payment.
func monthlyPayment(principal float64, annualRatePct float64, years int) float64 {
	r := annualRatePct / 100.0 / 12.0
	n := float64(years * 12)
	if r == 0 {
		return principal / n
	}
	return principal * r * math.Pow(1+r, n) / (math.Pow(1+r, n) - 1)
}

// ---------------------------------------------------------------------------
// Random credit score distribution (seeded, deterministic)
// ---------------------------------------------------------------------------

// normalScore generates a score from a clamped normal distribution.
func normalScore(rng *rand.Rand, mean, stddev float64, lo, hi int) int {
	for {
		v := rng.NormFloat64()*stddev + mean
		s := int(math.Round(v))
		if s >= lo && s <= hi {
			return s
		}
	}
}

// ---------------------------------------------------------------------------
// Salary from score (loosely correlated)
// ---------------------------------------------------------------------------

func salaryFromScore(rng *rand.Rand, score int) int {
	base := 28000.0 + float64(score-520)/310.0*292000.0
	jitter := rng.NormFloat64() * 15000
	s := int(math.Round(base+jitter) / 1000) * 1000
	if s < 28000 {
		s = 28000
	}
	if s > 320000 {
		s = 320000
	}
	return s
}

// ---------------------------------------------------------------------------
// Date helpers
// ---------------------------------------------------------------------------

// referenceDate is the "today" used throughout generation (2026-04-08).
var referenceDate = time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC)

func randomDate(rng *rand.Rand, lo, hi time.Time) time.Time {
	delta := hi.Unix() - lo.Unix()
	return lo.Add(time.Duration(rng.Int63n(delta)) * time.Second)
}

func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// ---------------------------------------------------------------------------
// Generator
// ---------------------------------------------------------------------------

func generate() []*Customer {
	rng := rand.New(rand.NewSource(42))

	// Pre-generate which customers have mortgages (~60, tier 1-4 only).
	// We'll track this as we build customers.

	customers := make([]*Customer, 100)

	// Counters to track distribution targets.
	mortgageCount := 0
	targetMortgages := 60

	for i := 0; i < 100; i++ {
		num := i + 1
		firstName := firstNames[i]
		lastName := lastNames[i]

		// Credit score — normal dist centered ~700, range 520-830.
		score := normalScore(rng, 700, 75, 520, 830)
		tier, tierLabel := scoreTier(score)

		salary := salaryFromScore(rng, score)

		// Age: 22-70
		age := 22 + rng.Intn(49)
		dobYear := 2026 - age
		dobMonth := 1 + rng.Intn(12)
		dobDay := 1 + rng.Intn(28)
		dob := fmt.Sprintf("%04d-%02d-%02d", dobYear, dobMonth, dobDay)

		// Customer since: between 1995 and 2024
		sinceYear := 1995 + rng.Intn(30)
		sinceMonth := 1 + rng.Intn(12)
		sinceDay := 1 + rng.Intn(28)
		since := fmt.Sprintf("%04d-%02d-%02d", sinceYear, sinceMonth, sinceDay)

		employer := employers[rng.Intn(len(employers))]
		jobTitle := jobTitles[rng.Intn(len(jobTitles))]

		// Accounts
		var accounts []Account
		// Checking account — all 100 have one
		checkingBalance := 500.0 + rng.Float64()*14500
		checkingType := "Basic Checking"
		if checkingBalance > 5000 || tier <= 2 {
			checkingType = "Premium Checking"
		}
		accounts = append(accounts, Account{
			ID:      fmt.Sprintf("ACC-%05d1", num),
			Type:    checkingType,
			Balance: math.Round(checkingBalance*100) / 100,
		})

		// Savings account — ~90 have one
		hasSavings := rng.Intn(10) < 9
		if hasSavings {
			savingsBalance := 1000.0 + rng.Float64()*float64(salary)/3
			savingsType := "Basic Savings"
			apy := savingsAPY[tier]
			if tier <= 2 {
				savingsType = "High-Yield Savings"
			}
			accounts = append(accounts, Account{
				ID:      fmt.Sprintf("ACC-%05d2", num),
				Type:    savingsType,
				Balance: math.Round(savingsBalance*100) / 100,
				APY:     apy,
			})
		}

		// Credit cards — ~85 have at least one
		var cards []CreditCard
		hasCard := rng.Intn(100) < 85
		if hasCard {
			card := buildCard(rng, num, tier, salary, false)
			cards = append(cards, card)

			// ~5 have multiple cards (tier 1-2 only)
			if tier <= 2 && rng.Intn(100) < 25 {
				card2 := buildCard(rng, num, tier, salary, true)
				cards = append(cards, card2)
			}
		}

		// Mortgage — ~60 total, only tier 1-4
		var mort *Mortgage
		if tier <= 4 && mortgageCount < targetMortgages {
			// probability adjusts so we hit ~60 across 92 tier 1-4 customers
			prob := float64(targetMortgages-mortgageCount) / float64(100-i)
			if rng.Float64() < prob {
				mort = buildMortgage(rng, tier, salary, num)
				mortgageCount++
			}
		}

		// Transactions: 10-20 per customer
		txns := buildTransactions(rng, num, salary, accounts, cards)

		// Notes
		notes := buildNotes(rng, num, tier, since, cards)

		// Active flags — first 10 customers get special notes
		if num <= 10 {
			flags := []string{
				"ALERT: Pending fraud investigation — account flagged 2026-03-15",
				"NOTE: Active dispute filed — unauthorized transaction 2026-02-28",
				"ALERT: Payment overdue 47 days — collections contact initiated",
				"NOTE: Fraud alert placed by customer 2026-01-10",
				"ALERT: High-risk transaction pattern detected — review required",
				"NOTE: Chargeback dispute in progress",
				"ALERT: Identity verification hold active",
				"NOTE: Overdue minimum payment — 3rd notice sent",
				"ALERT: Account under compliance review",
				"NOTE: Disputed charge $1,247.88 — merchant contacted",
			}
			notes = append(notes, flags[num-1])
		}

		customers[i] = &Customer{
			Num:           num,
			ID:            fmt.Sprintf("CUST-%05d", num),
			FirstName:     firstName,
			LastName:      lastName,
			Age:           age,
			DOB:           dob,
			Employer:      employer,
			JobTitle:      jobTitle,
			Salary:        salary,
			CreditScore:   score,
			Tier:          tier,
			TierLabel:     tierLabel,
			RiskRating:    tierRisk(tier),
			CustomerSince: since,
			Accounts:      accounts,
			Cards:         cards,
			Mortgage:      mort,
			Transactions:  txns,
			Notes:         notes,
		}
	}

	return customers
}

func buildCard(rng *rand.Rand, custNum, tier, salary int, secondary bool) CreditCard {
	var product string
	var apr float64

	switch {
	case tier <= 2:
		if secondary {
			// Secondary card for T1-2: use Cashback
			product = "Cashback Card"
			apr = ccAPRs["Cashback Card"][tier]
		} else {
			product = "Platinum Rewards Card"
			apr = ccAPRs["Platinum Rewards Card"][tier]
		}
	case tier == 3:
		product = "Cashback Card"
		apr = ccAPRs["Cashback Card"][3]
	default:
		product = "Secured Card"
		apr = ccAPRs["Secured Card"][tier]
	}

	pct := creditLimitPct[tier]
	if pct == 0 {
		pct = 0.10
	}
	rawLimit := float64(salary) * pct
	// Round to nearest $500
	limit := int(math.Round(rawLimit/500) * 500)
	if tier == 4 && limit > 10000 {
		limit = 10000
	}
	if secondary {
		limit = int(float64(limit) * 0.6)
		limit = int(math.Round(float64(limit)/500) * 500)
	}

	bal := math.Round(rng.Float64()*float64(limit)*0.4*100) / 100

	histories := []string{
		"Excellent (never missed)",
		"Good (1 late payment in 24 months)",
		"Excellent (never missed)",
		"Good (always on time)",
		"Excellent (never missed)",
	}
	history := histories[rng.Intn(len(histories))]

	suffix := ""
	if secondary {
		suffix = "B"
	}

	return CreditCard{
		ID:      fmt.Sprintf("CC-%05d%s", custNum, suffix),
		Product: product,
		Limit:   limit,
		Balance: bal,
		APR:     apr,
		History: history,
	}
}

func buildMortgage(rng *rand.Rand, tier, salary, custNum int) *Mortgage {
	// Select mortgage type
	types := []string{"30yr Fixed", "15yr Fixed"}
	// Tier 1-3 also eligible for ARM
	if tier <= 3 {
		types = append(types, "5/1 ARM")
	}
	mtype := types[rng.Intn(len(types))]

	rates, ok := mortgageRates[tier]
	if !ok {
		return nil
	}
	rate, ok := rates[mtype]
	if !ok {
		// Fallback to 30yr Fixed
		mtype = "30yr Fixed"
		rate = rates[mtype]
	}

	// Principal: 2-5x salary
	multiplier := 2.0 + rng.Float64()*3.0
	principal := math.Round(float64(salary)*multiplier/5000) * 5000

	years := 30
	if mtype == "15yr Fixed" {
		years = 15
	} else if mtype == "5/1 ARM" {
		years = 30
	}

	pmt := monthlyPayment(principal, rate, years)
	pmt = math.Round(pmt*100) / 100

	// Remaining term: random between 3 and max years
	remaining := 3 + rng.Intn(years-2)

	// Property address
	streetNum := 100 + rng.Intn(900)
	street := streetNames[rng.Intn(len(streetNames))]
	city := cities[rng.Intn(len(cities))]
	property := fmt.Sprintf("%d %s, %s", streetNum, street, city)

	return &Mortgage{
		Type:          mtype,
		Rate:          rate,
		Principal:     principal,
		Property:      property,
		MonthlyPmt:    pmt,
		RemainingTerm: remaining,
	}
}

func buildTransactions(rng *rand.Rand, custNum, salary int, accounts []Account, cards []CreditCard) []Transaction {
	numTxns := 10 + rng.Intn(11) // 10-20

	var txns []Transaction
	hasCard := len(cards) > 0

	// Base date: 30 days back from reference
	baseDate := referenceDate.AddDate(0, -1, 0)

	// Add 1-2 direct deposits
	depositMonthly := float64(salary) / 12
	biweekly := math.Round(depositMonthly/2*100) / 100
	depositDates := []int{1, 15}
	for _, dd := range depositDates {
		d := time.Date(referenceDate.Year(), referenceDate.Month(), dd, 0, 0, 0, 0, time.UTC)
		if d.Before(baseDate) {
			d = d.AddDate(0, 1, 0)
		}
		employer := "Employer"
		if len(accounts) > 0 {
			// Get employer name from outer scope (approximated)
			employer = "Direct Deposit"
		}
		txns = append(txns, Transaction{
			Date:        formatDate(d),
			Description: employer,
			Amount:      biweekly,
			Account:     "Checking",
		})
	}

	// Fill remaining with merchant transactions
	remaining := numTxns - len(txns)
	for j := 0; j < remaining; j++ {
		merchant := merchants[rng.Intn(len(merchants))]
		amount := math.Round((5.0+rng.Float64()*495)*100) / 100
		daysBack := rng.Intn(30)
		date := referenceDate.AddDate(0, 0, -daysBack)

		acctLabel := "Checking"
		if hasCard && rng.Intn(3) < 2 {
			acctLabel = "Credit Card"
		}

		txns = append(txns, Transaction{
			Date:        formatDate(date),
			Description: merchant,
			Amount:      -amount,
			Account:     acctLabel,
		})
	}

	// Sort by date descending (simple bubble sort — small slice)
	for a := 0; a < len(txns)-1; a++ {
		for b := 0; b < len(txns)-a-1; b++ {
			if txns[b].Date < txns[b+1].Date {
				txns[b], txns[b+1] = txns[b+1], txns[b]
			}
		}
	}

	return txns
}

func buildNotes(rng *rand.Rand, custNum, tier int, since string, cards []CreditCard) []string {
	var notes []string

	// Contact preference
	prefs := []string{"email", "phone", "mail", "mobile app"}
	notes = append(notes, fmt.Sprintf("Preferred contact: %s", prefs[rng.Intn(len(prefs))]))

	// Loyalty discount eligibility
	sinceYear := 0
	fmt.Sscanf(since[:4], "%d", &sinceYear)
	yearsCustomer := 2026 - sinceYear
	if yearsCustomer >= 10 {
		notes = append(notes, "Eligible for loyalty rate discount (0.250%)")
	} else if yearsCustomer >= 5 {
		notes = append(notes, "Eligible for loyalty rate discount (0.125%)")
	}

	return notes
}

// ---------------------------------------------------------------------------
// Markdown rendering
// ---------------------------------------------------------------------------

func renderCustomer(c *Customer) string {
	var sb strings.Builder

	// Header
	fmt.Fprintf(&sb, "# %s %s — Customer Profile\n\n", c.FirstName, c.LastName)

	// Personal Information
	sb.WriteString("## Personal Information\n\n")
	fmt.Fprintf(&sb, "- **Customer ID:** %s\n", c.ID)
	fmt.Fprintf(&sb, "- **Age:** %d | **DOB:** %s\n", c.Age, c.DOB)
	fmt.Fprintf(&sb, "- **Employment:** %s at %s\n", c.JobTitle, c.Employer)
	fmt.Fprintf(&sb, "- **Annual Salary:** $%s\n", formatInt(c.Salary))
	fmt.Fprintf(&sb, "- **Credit Score:** %d (%s)\n", c.CreditScore, c.TierLabel)
	fmt.Fprintf(&sb, "- **Customer Since:** %s\n", c.CustomerSince)
	fmt.Fprintf(&sb, "- **Risk Rating:** %s\n", c.RiskRating)
	sb.WriteString("\n")

	// Accounts
	sb.WriteString("## Accounts\n\n")
	for _, acct := range c.Accounts {
		fmt.Fprintf(&sb, "### %s (%s)\n\n", acct.Type, acct.ID)
		fmt.Fprintf(&sb, "- **Balance:** $%.2f\n", acct.Balance)
		if acct.APY > 0 {
			fmt.Fprintf(&sb, "- **APY:** %.2f%%\n", acct.APY)
		}
		sb.WriteString("\n")
	}

	// Credit Cards
	if len(c.Cards) > 0 {
		sb.WriteString("## Credit Cards\n\n")
		for _, card := range c.Cards {
			fmt.Fprintf(&sb, "### %s (%s)\n\n", card.Product, card.ID)
			fmt.Fprintf(&sb, "- **Credit Limit:** $%s\n", formatInt(card.Limit))
			fmt.Fprintf(&sb, "- **Current Balance:** $%.2f\n", card.Balance)
			fmt.Fprintf(&sb, "- **APR:** %.2f%%\n", card.APR)
			fmt.Fprintf(&sb, "- **Payment History:** %s\n", card.History)
			sb.WriteString("\n")
		}
	}

	// Mortgage
	if c.Mortgage != nil {
		m := c.Mortgage
		sb.WriteString("## Mortgage\n\n")
		fmt.Fprintf(&sb, "- **Type:** %s\n", m.Type)
		fmt.Fprintf(&sb, "- **Rate:** %.3f%%\n", m.Rate)
		fmt.Fprintf(&sb, "- **Original Principal:** $%s\n", formatInt(int(m.Principal)))
		fmt.Fprintf(&sb, "- **Property:** %s\n", m.Property)
		fmt.Fprintf(&sb, "- **Monthly Payment:** $%.2f\n", m.MonthlyPmt)
		fmt.Fprintf(&sb, "- **Remaining Term:** %d years\n", m.RemainingTerm)
		sb.WriteString("\n")
	}

	// Transactions
	sb.WriteString("## Recent Transactions (Last 30 Days)\n\n")
	sb.WriteString("| Date | Description | Amount | Account |\n")
	sb.WriteString("|------|-------------|--------|--------|\n")
	for _, t := range c.Transactions {
		amtStr := ""
		if t.Amount >= 0 {
			amtStr = fmt.Sprintf("+$%.2f", t.Amount)
		} else {
			amtStr = fmt.Sprintf("-$%.2f", -t.Amount)
		}
		fmt.Fprintf(&sb, "| %s | %s | %s | %s |\n", t.Date, t.Description, amtStr, t.Account)
	}
	sb.WriteString("\n")

	// Notes
	sb.WriteString("## Notes\n\n")
	for _, n := range c.Notes {
		fmt.Fprintf(&sb, "- %s\n", n)
	}
	sb.WriteString("\n")

	return sb.String()
}

func formatInt(n int) string {
	s := fmt.Sprintf("%d", n)
	result := ""
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result += ","
		}
		result += string(ch)
	}
	return result
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	outDir := filepath.Join("content", "customers")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir error: %v\n", err)
		os.Exit(1)
	}

	customers := generate()

	// Summary counters
	var mortgages, ccHolders, multiCard, savings, flagged int
	tierCounts := make(map[int]int)

	for _, c := range customers {
		tierCounts[c.Tier]++
		if c.Mortgage != nil {
			mortgages++
		}
		if len(c.Cards) > 0 {
			ccHolders++
		}
		if len(c.Cards) > 1 {
			multiCard++
		}
		for _, a := range c.Accounts {
			if strings.Contains(a.Type, "Savings") {
				savings++
				break
			}
		}
		for _, n := range c.Notes {
			if strings.Contains(n, "ALERT") || strings.Contains(n, "flag") || strings.Contains(n, "dispute") || strings.Contains(n, "overdue") || strings.Contains(n, "fraud") {
				flagged++
				break
			}
		}

		// Write file
		md := renderCustomer(c)
		filename := strings.ToLower(c.FirstName) + "-" + strings.ToLower(c.LastName) + ".md"
		path := filepath.Join(outDir, filename)
		if err := os.WriteFile(path, []byte(md), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write error %s: %v\n", path, err)
			os.Exit(1)
		}
	}

	fmt.Println("=== Generation Complete ===")
	fmt.Printf("Total customers: %d\n", len(customers))
	fmt.Printf("Tier 1 (Excellent 800-830): %d\n", tierCounts[1])
	fmt.Printf("Tier 2 (Very Good 740-799): %d\n", tierCounts[2])
	fmt.Printf("Tier 3 (Good 670-739):      %d\n", tierCounts[3])
	fmt.Printf("Tier 4 (Fair 580-669):      %d\n", tierCounts[4])
	fmt.Printf("Tier 5 (Poor 520-579):      %d\n", tierCounts[5])
	fmt.Printf("With mortgages:  %d\n", mortgages)
	fmt.Printf("With credit cards: %d\n", ccHolders)
	fmt.Printf("Multiple cards:  %d\n", multiCard)
	fmt.Printf("With savings:    %d\n", savings)
	fmt.Printf("Active flags:    %d\n", flagged)
	fmt.Printf("Files written to: %s/\n", outDir)
}
