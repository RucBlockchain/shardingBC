## Tx格式
```javascript
{
    "Txtype": "",//string, 交易类型
    "Sender": "",//string, 发送方分片
    "Receiver": "",//string, 接收方分片
    "ID": "",//[sha256.Size]byte, 未知
    "Content": "",//string, 交易内容，16进制编码
    "TxSignature": ""//string,交易签名，16进制编码
}
```  
### 交易内容格式说明
content的格式： {sender}_{receiver}_{amount}  
content的交易由三部分组成：转出方、转入方、金额  
其中转出方和转入方是来ecdsa公钥的string形式  

### ecdsa的坐标与string转换
转换方法遵循asn1(Abstract Syntax Notation One)格式  
> golang官方包 encoding/asn1

将坐标ecdsaCoordinate直接marshal为byte数据，然后再将byte数据按16进制编码转换为string类型  
ecdsa的坐标结构：  
```golang
type struct ecdsaCoordinate{
    X, Y *big.Int
}
```  
（ps：ecdsa的签名结果也是一个坐标，其中的R对应ecdsaCoordinate.X，S对应ecdsaCoordinate.Y）
