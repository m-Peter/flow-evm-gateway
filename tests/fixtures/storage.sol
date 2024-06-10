// SPDX-License-Identifier: GPL-3.0

pragma solidity >=0.8.2 <0.9.0;

contract Storage {

    uint256 public min;
    uint256 public avg;
    uint256 public max;

    function storeMinMax(uint256 minNum, uint256 average, uint256 maxNum) public {
        min = minNum;
        avg = average;
        max = maxNum;
    }
}
