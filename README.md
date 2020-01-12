# Sharding Blockchain
高性能区块链分片系统，在保证交易安全性的前提下实现横向扩容，显著提高系统TPS。

## 内容
+ 设计并实现跨片共识协议
+ Leader定期更替协议
+ 跨片共识系统的安全性
+ 大规模实验压力测试

## 创新
+ 保证交易的最终一致性
+ 用ETCD维护分片间的通信
+ 记录每个跨片交易，收到最终确认消息后再删除
+ 增加检查点区块，加快leader更换时的跨片交易记录恢复

## 开发
### Tx格式
```javascript
{
    "Txtype"         // string, 交易类型
    "Sender"         // string, 发送方分片
    "Receiver"       // string, 接收方分片
    "ID"             // [sha256.Size]byte, 未知
    "Content"        // string, 交易内容，16进制编码
    "TxSignature"    // string, 交易签名，16进制编码
}
```  

### 交易内容格式说明
+ content格式： {sender} _ {receiver} _ {amount}   
+ content的交易由三部分组成：转出方、转入方、金额，其中转出方和转入方是来ecdsa公钥的string形式  

### ECDSA的坐标与string转换
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
