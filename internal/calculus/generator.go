// Package calculus provides calculus problem generation for OTP.
package calculus

import (
	"fmt"
	"math/rand"
)

// ProblemResult contains the generated calculus problem.
type ProblemResult struct {
	OTP          int    // The correct 6-digit answer
	A            int    // Coefficient of x^2
	B            int    // Coefficient of x
	C            int    // Constant term
	K            int    // Evaluation point
	ProblemLatex string // LaTeX representation
}

// Generator creates calculus problems.
type Generator struct{}

// NewGenerator creates a new calculus problem generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate creates a new calculus problem where f'(k) = OTP.
// The polynomial is f(x) = ax^2 + bx + c, so f'(x) = 2ax + b.
// We solve for coefficients such that f'(k) = 2ak + b = OTP.
func (g *Generator) Generate() (*ProblemResult, error) {
	// Step 1: Generate 6-digit OTP (100000-999999)
	otp := rand.Intn(900000) + 100000

	// Step 2: Select evaluation point (nice numbers for mental math)
	kOptions := []int{10, 20, 50, 100, 200}
	k := kOptions[rand.Intn(len(kOptions))]

	// Step 3: Select coefficient a (small positive integer)
	// We need 2ak < otp so that b > 0
	aOptions := []int{1, 2, 3, 4, 5}
	var a, b int

	// Shuffle aOptions to add randomness
	rand.Shuffle(len(aOptions), func(i, j int) {
		aOptions[i], aOptions[j] = aOptions[j], aOptions[i]
	})

	// Try to find valid a where b > 0
	for _, candidateA := range aOptions {
		candidateB := otp - 2*candidateA*k
		if candidateB > 0 {
			a = candidateA
			b = candidateB
			break
		}
	}

	// If no valid a found (OTP too small for chosen k), use smaller k
	if a == 0 {
		k = 10
		a = 1
		b = otp - 2*a*k
		// If still negative, just use a=1 and adjust
		if b <= 0 {
			a = 1
			b = 1
			// Regenerate OTP to match
			otp = 2*a*k + b
		}
	}

	// Step 4: Generate arbitrary constant c (makes problem harder to reverse)
	c := rand.Intn(100) + 1

	// Step 5: Generate LaTeX problem text
	latex := g.generateLatex(a, b, c, k)

	return &ProblemResult{
		OTP:          otp,
		A:            a,
		B:            b,
		C:            c,
		K:            k,
		ProblemLatex: latex,
	}, nil
}

// generateLatex creates the LaTeX representation of the problem.
func (g *Generator) generateLatex(a, b, c, k int) string {
	// Build f(x) = ax^2 + bx + c
	var fxParts []string

	// ax^2 term
	if a == 1 {
		fxParts = append(fxParts, "x^2")
	} else {
		fxParts = append(fxParts, fmt.Sprintf("%dx^2", a))
	}

	// bx term
	if b == 1 {
		fxParts = append(fxParts, "+ x")
	} else {
		fxParts = append(fxParts, fmt.Sprintf("+ %dx", b))
	}

	// c term
	fxParts = append(fxParts, fmt.Sprintf("+ %d", c))

	fx := ""
	for i, part := range fxParts {
		if i == 0 {
			fx = part
		} else {
			fx += " " + part
		}
	}

	return fmt.Sprintf("f(x) = %s を微分し、x = %d での値を求めよ", fx, k)
}

// Verify checks if the given answer matches the OTP.
func (g *Generator) Verify(answer, otp int) bool {
	return answer == otp
}
