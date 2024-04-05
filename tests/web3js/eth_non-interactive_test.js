const { Web3 } = require('web3')
const web3Utils = require('web3-utils')
const { assert } = require('chai')

const web3 = new Web3("http://localhost:8545")

const eoaAccount = web3.eth.accounts.privateKeyToAccount("0xf6d5333177711e562cabf1f311916196ee6ffc2a07966d9d4628094073bd5442")
const fundedAmount = 5.0
const startBlockHeight = 3 // start block height after setup accounts
const serviceEOA = "0xfacf71692421039876a5bb4f10ef7a439d8ef61e" // configured account as gw service
const successStatus = 1n

it('get chain ID', async() => {
    let chainID = await web3.eth.getChainId()
    assert.isDefined(chainID)
    assert.equal(chainID, 646n)
})

it('get block', async () => {
    let height = await web3.eth.getBlockNumber()
    assert.equal(height, startBlockHeight)

    let block = await web3.eth.getBlock(height)
    assert.notDeepEqual(block, {})
    assert.isString(block.hash)
    assert.isString(block.parentHash)
    assert.isString(block.logsBloom)

    let blockHash = await web3.eth.getBlock(block.hash)
    assert.deepEqual(block, blockHash)

    // get block count and uncle count
    let txCount = await web3.eth.getBlockTransactionCount(startBlockHeight)
    let uncleCount = await web3.eth.getBlockUncleCount(startBlockHeight)

    assert.equal(txCount, 1n)
    assert.equal(uncleCount, 0n)

    // get block transaction
    let tx = await web3.eth.getTransactionFromBlock(startBlockHeight, 0)
    assert.isNotNull(tx)
    assert.equal(tx.blockNumber, block.number)
    assert.equal(tx.blockHash, block.hash)
    assert.isString(tx.hash)

    // not existing transaction
    let no = await web3.eth.getTransactionFromBlock(startBlockHeight, 1)
    assert.isNull(no)
})

it('get balance', async() => {
    let wei = await web3.eth.getBalance(eoaAccount.address)
    assert.isNotNull(wei)

    let flow = web3Utils.fromWei(wei, 'ether')
    assert.equal(parseFloat(flow), fundedAmount)

    let weiAtBlock = await web3.eth.getBalance(eoaAccount.address, startBlockHeight)
    assert.equal(wei, weiAtBlock)
})

it('get code', async() => {
    let code = await web3.eth.getCode(eoaAccount.address)
    assert.equal(code, "0x") // empty
})

it('get coinbase', async() => {
    let coinbase = await web3.eth.getCoinbase()
    assert.equal(coinbase, serviceEOA) // e2e configured account
})

it('get gas price', async() => {
    let gasPrice = await web3.eth.getGasPrice()
    assert.equal(gasPrice, 0n) // 0 by default in tests
})

it('get transaction', async() => {
    let blockTx = await web3.eth.getTransactionFromBlock(startBlockHeight, 0)
    assert.isNotNull(blockTx)

    let tx = await web3.eth.getTransaction(blockTx.hash)
    assert.deepEqual(blockTx, tx)
    assert.isString(tx.hash)
    assert.equal(tx.blockNumber, startBlockHeight)
    assert.isAbove(parseInt(tx.gas), 1)
    assert.isNotEmpty(tx.from)
    assert.isNotEmpty(tx.r)
    assert.isNotEmpty(tx.s)
    assert.equal(tx.transactionIndex, 0)

    let rcp = await web3.eth.getTransactionReceipt(tx.hash)
    assert.isNotEmpty(rcp)
    assert.equal(rcp.blockHash, blockTx.blockHash)
    assert.equal(rcp.blockNumber, startBlockHeight)
    assert.equal(rcp.from, tx.from.toLowerCase()) // todo checksum format
    assert.equal(rcp.to, tx.to.toLowerCase()) // todo checksum format
    assert.equal(rcp.cumulativeGasUsed, tx.gas) // todo check
    assert.equal(rcp.transactionHash, tx.hash)
    assert.equal(rcp.status, successStatus)
    assert.equal(rcp.gasUsed, tx.gas)
})
