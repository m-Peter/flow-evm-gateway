import EVM
import FungibleToken
import FlowToken

transaction(amount: UFix64) {
    let sentVault: @FlowToken.Vault
    let addrVault: @FlowToken.Vault
    let addr1Vault: @FlowToken.Vault
    let auth: auth(Storage) &Account

    prepare(signer: auth(Storage) &Account) {
        let vaultRef = signer.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(
            from: /storage/flowTokenVault
        ) ?? panic("Could not borrow reference to the owner's Vault!")

        self.sentVault <- vaultRef.withdraw(amount: 1000.0) as! @FlowToken.Vault
        self.addrVault <- vaultRef.withdraw(amount: 1000.0) as! @FlowToken.Vault
        self.addr1Vault <- vaultRef.withdraw(amount: 1000.0) as! @FlowToken.Vault
        self.auth = signer
    }

    execute {
        let from: String = "0x4c8D290a1B368ac4728d83a9e8321fC3af2b39b1"
        let fromAddr = EVM.addressFromString(from)
        fromAddr.deposit(from: <-self.addrVault)

        let anotherAddr = EVM.addressFromString("0x3CA7971B5be71bcD2DB6cEDf667F0AE5e5022fEd")
        anotherAddr.deposit(from: <-self.addr1Vault)
        let account <- EVM.createCadenceOwnedAccount()
        log(account.address())
        account.deposit(from: <-self.sentVault)

        log(account.balance())
        self.auth.storage.save<@EVM.CadenceOwnedAccount>(<-account, to: StoragePath(identifier: "evm")!)
    }
}
