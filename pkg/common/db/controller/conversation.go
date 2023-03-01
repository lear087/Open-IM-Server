package controller

import (
	"OpenIM/pkg/common/constant"
	"OpenIM/pkg/common/db/cache"
	"OpenIM/pkg/common/db/relation"
	relationTb "OpenIM/pkg/common/db/table/relation"
	"OpenIM/pkg/common/db/tx"
	"OpenIM/pkg/utils"
	"context"
	"encoding/json"
)

type ConversationDatabase interface {
	//GetUserIDExistConversation 获取拥有该会话的的用户ID列表
	GetUserIDExistConversation(ctx context.Context, userIDList []string, conversationID string) ([]string, error)
	//UpdateUserConversationFiled 更新用户该会话的属性信息
	UpdateUsersConversationFiled(ctx context.Context, UserIDList []string, conversationID string, args map[string]interface{}) error
	//CreateConversation 创建一批新的会话
	CreateConversation(ctx context.Context, conversations []*relationTb.ConversationModel) error
	//SyncPeerUserPrivateConversation 同步对端私聊会话内部保证事务操作
	SyncPeerUserPrivateConversationTx(ctx context.Context, conversation *relationTb.ConversationModel) error
	//FindConversations 根据会话ID获取某个用户的多个会话
	FindConversations(ctx context.Context, ownerUserID string, conversationIDs []string) ([]*relationTb.ConversationModel, error)
	//FindRecvMsgNotNotifyUserIDs 获取超级大群开启免打扰的用户ID
	FindRecvMsgNotNotifyUserIDs(ctx context.Context, groupID string) ([]string, error)
	//GetUserAllConversation 获取一个用户在服务器上所有的会话
	GetUserAllConversation(ctx context.Context, ownerUserID string) ([]*relationTb.ConversationModel, error)
	//SetUserConversations 设置用户多个会话属性，如果会话不存在则创建，否则更新,内部保证原子性
	SetUserConversations(ctx context.Context, ownerUserID string, conversations []*relationTb.ConversationModel) error
	//SetUsersConversationFiledTx 设置多个用户会话关于某个字段的更新操作，如果会话不存在则创建，否则更新，内部保证事务操作
	SetUsersConversationFiledTx(ctx context.Context, userIDList []string, conversation *relationTb.ConversationModel, filedMap map[string]interface{}) error
}

func NewConversationDatabase(conversation relation.Conversation, cache cache.ConversationCache, tx tx.Tx) ConversationDatabase {
	return &ConversationDataBase{
		conversationDB: conversation,
		cache:          cache,
		tx:             tx,
	}
}

type ConversationDataBase struct {
	conversationDB relation.Conversation
	cache          cache.ConversationCache
	tx             tx.Tx
}

func (c *ConversationDataBase) SetUsersConversationFiledTx(ctx context.Context, userIDList []string, conversation *relationTb.ConversationModel, filedMap map[string]interface{}) error {
	return c.tx.Transaction(func(tx any) error {
		conversationTx := c.conversationDB.NewTx(tx)
		haveUserID, err := conversationTx.FindUserID(ctx, userIDList, conversation.ConversationID)
		if err != nil {
			return err
		}
		if len(haveUserID) > 0 {
			err = conversationTx.UpdateByMap(ctx, haveUserID, conversation.ConversationID, filedMap)
			if err != nil {
				return err
			}
		}
		NotUserID := utils.DifferenceString(haveUserID, userIDList)
		var cList []*relationTb.ConversationModel
		for _, v := range NotUserID {
			temp := new(relationTb.ConversationModel)
			if err := utils.CopyStructFields(temp, conversation); err != nil {
				return err
			}
			temp.OwnerUserID = v
			cList = append(cList, temp)
		}
		err = conversationTx.Create(ctx, cList)
		if err != nil {
			return err
		}
		if len(NotUserID) > 0 {
			err = c.cache.DelUsersConversationIDs(ctx, NotUserID)
			if err != nil {
				return err
			}
		}
		err = c.cache.DelUsersConversation(ctx, haveUserID, conversation.ConversationID)
		if err != nil {
			return err
		}
		return nil
	})
}

func (c *ConversationDataBase) GetUserIDExistConversation(ctx context.Context, userIDList []string, conversationID string) ([]string, error) {
	panic("implement me")
}

func (c *ConversationDataBase) UpdateUsersConversationFiled(ctx context.Context, UserIDList []string, conversationID string, args map[string]interface{}) error {
	panic("implement me")
}

func (c *ConversationDataBase) CreateConversation(ctx context.Context, conversations []*relationTb.ConversationModel) error {
	return c.tx.Transaction(func(tx any) error {
		if err := c.conversationDB.NewTx(tx).Create(ctx, conversations); err != nil {
			return err
		}
		// clear cache
		return nil
	})
}

func (c *ConversationDataBase) SyncPeerUserPrivateConversationTx(ctx context.Context, conversation *relationTb.ConversationModel) error {
	return c.tx.Transaction(func(tx any) error {
		userIDList := []string{conversation.OwnerUserID, conversation.UserID}
		conversationTx := c.conversationDB.NewTx(tx)
		haveUserID, err := conversationTx.FindUserID(ctx, userIDList, conversation.ConversationID)
		if err != nil {
			return err
		}
		filedMap := map[string]interface{}{"is_private_chat": conversation.IsPrivateChat}
		if len(haveUserID) > 0 {
			err = conversationTx.UpdateByMap(ctx, haveUserID, conversation.ConversationID, filedMap)
			if err != nil {
				return err
			}
		}
		NotUserID := utils.DifferenceString(haveUserID, userIDList)
		var cList []*relationTb.ConversationModel
		for _, v := range NotUserID {
			temp := new(relationTb.ConversationModel)
			if v == conversation.UserID {
				temp.OwnerUserID = conversation.UserID
				temp.ConversationID = utils.GetConversationIDBySessionType(conversation.OwnerUserID, constant.SingleChatType)
				temp.ConversationType = constant.SingleChatType
				temp.UserID = conversation.OwnerUserID
				temp.IsPrivateChat = conversation.IsPrivateChat
			} else {
				if err := utils.CopyStructFields(temp, conversation); err != nil {
					return err
				}
				temp.OwnerUserID = v
			}
			cList = append(cList, temp)
		}
		if len(NotUserID) > 0 {
			err = c.conversationDB.Create(ctx, cList)
			if err != nil {
				return err
			}
		}
		err = c.cache.DelUsersConversationIDs(ctx, NotUserID)
		if err != nil {
			return err
		}
		err = c.cache.DelUsersConversation(ctx, haveUserID, conversation.ConversationID)
		if err != nil {
			return err
		}
		return nil
	})
}

func (c *ConversationDataBase) FindConversations(ctx context.Context, ownerUserID string, conversationIDs []string) ([]*relationTb.ConversationModel, error) {
	getConversation := func() (string, error) {
		conversationList, err := c.conversationDB.Find(ctx, ownerUserID, conversationIDs)
		if err != nil {
			return "", utils.Wrap(err, "get failed")
		}
		bytes, err := json.Marshal(conversationList)
		if err != nil {
			return "", utils.Wrap(err, "Marshal failed")
		}
		return string(bytes), nil
	}
	return c.cache.GetConversations(ctx, ownerUserID, conversationIDs, getConversation)
}

func (c *ConversationDataBase) GetConversation(ctx context.Context, ownerUserID string, conversationID string) (*relationTb.ConversationModel, error) {
	getConversation := func() (string, error) {
		conversationList, err := c.conversationDB.Take(ctx, ownerUserID, conversationID)
		if err != nil {
			return "", utils.Wrap(err, "get failed")
		}
		bytes, err := json.Marshal(conversationList)
		if err != nil {
			return "", utils.Wrap(err, "Marshal failed")
		}
		return string(bytes), nil
	}
	return c.cache.GetConversation(ctx, ownerUserID, conversationID, getConversation)
}

func (c *ConversationDataBase) GetUserAllConversation(ctx context.Context, ownerUserID string) ([]*relationTb.ConversationModel, error) {
	getConversationIDList := func() (string, error) {
		conversationIDList, err := c.conversationDB.FindUserIDAllConversationID(ctx, ownerUserID)
		if err != nil {
			return "", utils.Wrap(err, "getConversationIDList failed")
		}
		bytes, err := json.Marshal(conversationIDList)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		return string(bytes), nil
	}
	conversationIDList, err := c.cache.GetUserConversationIDs(ctx, ownerUserID, getConversationIDList)
	if err != nil {
		return nil, err
	}
	var conversations []*relationTb.ConversationModel
	for _, conversationID := range conversationIDList {
		conversation, err := c.GetConversation(ctx, ownerUserID, conversationID)
		if err != nil {
			return nil, utils.Wrap(err, "GetConversation failed")
		}
		conversations = append(conversations, conversation)
	}
	return conversations, nil
}

func (c *ConversationDataBase) SetUserConversations(ctx context.Context, ownerUserID string, conversations []*relationTb.ConversationModel) error {
	return c.tx.Transaction(func(tx any) error {
		var conversationIDList []string
		for _, conversation := range conversations {
			conversationIDList = append(conversationIDList, conversation.ConversationID)
		}
		conversationTx := c.conversationDB.NewTx(tx)
		haveConversations, err := conversationTx.Find(ctx, ownerUserID, conversationIDList)
		if err != nil {
			return err
		}
		if len(haveConversations) > 0 {
			err = conversationTx.Update(ctx, conversations)
			if err != nil {
				return err
			}
		}
		var haveConversationID []string
		for _, conversation := range haveConversations {
			haveConversationID = append(haveConversationID, conversation.ConversationID)
		}

		NotConversationID := utils.DifferenceString(haveConversationID, conversationIDList)
		var NotConversations []*relationTb.ConversationModel
		for _, conversation := range conversations {
			if !utils.IsContain(conversation.ConversationID, haveConversationID) {
				NotConversations = append(NotConversations, conversation)
			}
		}
		if len(NotConversations) > 0 {
			err = c.conversationDB.Create(ctx, NotConversations)
			if err != nil {
				return err
			}
		}
		err = c.cache.DelUsersConversationIDs(ctx, NotConversationID)
		if err != nil {
			return err
		}
		err = c.cache.DelUserConversations(ctx, ownerUserID, haveConversationID)
		if err != nil {
			return err
		}
		return nil
	})
}

func (c *ConversationDataBase) FindRecvMsgNotNotifyUserIDs(ctx context.Context, groupID string) ([]string, error) {
	return c.conversationDB.FindRecvMsgNotNotifyUserIDs(ctx, groupID)
}
