import EVM

access(all)
fun main(hexEncodedTx: String, hexEncodedAddress: String): EVM.Result {
    let addressBytes = hexEncodedAddress.decodeHex().toConstantSized<[UInt8; 20]>()!
    let address = EVM.EVMAddress(bytes: addressBytes)

    let txResult = EVM.dryRun(tx: hexEncodedTx.decodeHex(), from: address)
    log(txResult)
    return txResult
}
