package crypto

import (
	"fmt"
	"math/rand"

	"github.com/dedis/crypto/abstract"
)

func NeffShuffle(x []abstract.Point, base abstract.Point, suite abstract.Suite) ([]abstract.Point, abstract.Point) {

	for k := 0; k < len(x); k++ {
		fmt.Println("x[", k, "] = ", x[k])
	}

	//first, we shuffle the array
	x2 := make([]abstract.Point, len(x))

	shuffledIndices := rand.Perm(len(x))
	i := 0
	for j, _ := range shuffledIndices {
		x2[j] = x[i]
		i++
	}

	for k := 0; k < len(x2); k++ {
		fmt.Println("x2[", k, "] = ", x2[k])
	}

	//do the neff shuffle

	//1. pick a new base
	rand := suite.Cipher([]byte("randomStuff"))
	base2 := suite.Secret().Pick(rand)

	//3. multiply by the new base
	x3 := make([]abstract.Point, len(x2))
	for k := 0; k < len(x2); k++ {
		x3[k] = suite.Point().Mul(x2[k], base2)
	}

	//3. the final base (for points in x3) is base*base2
	baseFinal := suite.Point().Mul(base, base2)

	return x3, baseFinal
}

func NeffVerify() {

}
