import stainless.smartcontracts._
import stainless.lang.StaticChecks._
import stainless.annotation._

case class Candy(
  var initialCandies: Uint256,
  var remainingCandies: Uint256,
  var eatenCandies: Uint256
) extends Contract {

  def constructor(_candies: Uint256) = {
    initialCandies = _candies
    remainingCandies = _candies
    eatenCandies = Uint256.ZERO

    assert(invariant)
  }

  def eatCandy(candies: Uint256) = {
    require(invariant)
    dynRequire(candies <= remainingCandies)

    remainingCandies -= candies
    eatenCandies += candies

    assert(invariant)
  }

  @view
  def getRemainingCandies(): Uint256 = remainingCandies;

  @view
  private def invariant: Boolean = {
    eatenCandies <= initialCandies &&
    remainingCandies <= initialCandies &&
    initialCandies - eatenCandies == remainingCandies
  }
}

