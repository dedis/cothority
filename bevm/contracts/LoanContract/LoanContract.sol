pragma solidity ^0.4.24;

import "ERC20Token.sol";

contract LoanContract {
    // Fields
    address borrower;
    uint256 wantedAmount;
    uint256 premiumAmount;
    uint256 tokenAmount;
    string tokenName;
    ERC20Token tokenContractAddress;
    uint256 daysToLend;
    State currentState;
    uint256 start;
    address lender;
    State[] visitedStates;

    // Enumerations
    enum State{
        WaitingForPayback,
        Finished,
        Default,
        WaitingForData,
        WaitingForLender
    }


    // Constructor
    constructor (uint256 _wantedAmount, uint256 _interest, uint256 _tokenAmount, string _tokenName, ERC20Token _tokenContractAddress, uint256 _length) public {
      wantedAmount = _wantedAmount;
      premiumAmount = _interest;
      tokenAmount = _tokenAmount;
      tokenName = _tokenName;
      tokenContractAddress = _tokenContractAddress;
      daysToLend = _length;
      borrower = msg.sender;
      currentState = State.WaitingForData;
    }

    // Public functions
    function lend () public payable {
        if (!(msg.sender == borrower)) {
            if (currentState == State.WaitingForLender && msg.value >= wantedAmount) {
                lender = msg.sender;
                borrower.transfer(wantedAmount);
                currentState = State.WaitingForPayback;
                start = now;
            }

        }

    }

    function checkTokens () public {
        if (currentState == State.WaitingForData) {
            uint256 balance = tokenContractAddress.balanceOf(address(this));
            if (balance >= tokenAmount) {
                currentState = State.WaitingForLender;
            }

        }

    }

    function requestDefault () public {
        if (currentState == State.WaitingForPayback) {
            require(now > start + daysToLend, "error");
            require(msg.sender == lender, "error");
            uint256 balance = tokenContractAddress.balanceOf(address(this));
            tokenContractAddress.transfer(lender, balance);

            currentState = State.Default;
        }

    }

    function payback () public payable {
        require(address(this).balance >= msg.value, "error");
        require(msg.value >= premiumAmount + wantedAmount, "error");
        require(msg.sender == borrower, "error");
        if (currentState == State.WaitingForPayback) {
            lender.transfer(msg.value);
            uint256 balance = tokenContractAddress.balanceOf(address(this));
            tokenContractAddress.transfer(borrower, balance);

            currentState = State.Finished;
        }

    }

    // Private functions

}
