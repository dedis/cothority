pragma experimental ABIEncoderV2;
pragma solidity ^0.5.0;

contract ABIv2 {
    struct S {
        uint256 v1;
        uint256 v2;
    }

    function squares(uint256 limit) public view returns (S[] memory) {
        S[] memory result = new S[](limit);

        for (uint256 i = 0; i < limit; i++) {
            S memory s = S(i, i * i);
            result[i] = s;
        }

        return result;
    }
}
