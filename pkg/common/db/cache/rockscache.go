package cache

import (
	"Open_IM/pkg/common/constant"
	"Open_IM/pkg/common/db/mongo"
	"Open_IM/pkg/common/db/mysql"
	"Open_IM/pkg/common/log"
	"Open_IM/pkg/common/tracelog"
	"Open_IM/pkg/utils"
	"context"
	"encoding/json"
	"github.com/dtm-labs/rockscache"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
	"math/big"
	"sort"
	"strconv"
	"time"
)

const (
	userInfoCache       = "USER_INFO_CACHE:"
	friendRelationCache = "FRIEND_RELATION_CACHE:"
	blackListCache      = "BLACK_LIST_CACHE:"
	groupCache          = "GROUP_CACHE:"
	//groupInfoCache            = "GROUP_INFO_CACHE:"
	groupOwnerIDCache         = "GROUP_OWNER_ID:"
	joinedGroupListCache      = "JOINED_GROUP_LIST_CACHE:"
	groupMemberInfoCache      = "GROUP_MEMBER_INFO_CACHE:"
	groupAllMemberInfoCache   = "GROUP_ALL_MEMBER_INFO_CACHE:"
	allFriendInfoCache        = "ALL_FRIEND_INFO_CACHE:"
	joinedSuperGroupListCache = "JOINED_SUPER_GROUP_LIST_CACHE:"
	groupMemberListHashCache  = "GROUP_MEMBER_LIST_HASH_CACHE:"
	groupMemberNumCache       = "GROUP_MEMBER_NUM_CACHE:"
	conversationCache         = "CONVERSATION_CACHE:"
	conversationIDListCache   = "CONVERSATION_ID_LIST_CACHE:"
	extendMsgSetCache         = "EXTEND_MSG_SET_CACHE:"
	extendMsgCache            = "EXTEND_MSG_CACHE:"
)

const scanCount = 3000
const RandomExpireAdjustment = 0.2

func (rc *RcClient) DelKeys() {
	for _, key := range []string{groupCache, friendRelationCache, blackListCache, userInfoCache, groupInfoCacheKey, groupOwnerIDCache, joinedGroupListCache,
		groupMemberInfoCache, groupAllMemberInfoCache, allFriendInfoCache} {
		fName := utils.GetSelfFuncName()
		var cursor uint64
		var n int
		for {
			var keys []string
			var err error
			keys, cursor, err = rc.rdb.Scan(context.Background(), cursor, key+"*", scanCount).Result()
			if err != nil {
				panic(err.Error())
			}
			n += len(keys)
			// for each for redis cluster
			for _, key := range keys {
				if err = rc.rdb.Del(context.Background(), key).Err(); err != nil {
					log.NewError("", fName, key, err.Error())
					err = rc.rdb.Del(context.Background(), key).Err()
					if err != nil {
						panic(err.Error())
					}
				}
			}
			if cursor == 0 {
				break
			}
		}
	}
}

func (rc *Client) GetFriendIDListFromCache(ctx context.Context, userID string) (friendIDList []string, err error) {
	getFriendIDList := func() (string, error) {
		friendIDList, err := relation.GetFriendIDListByUserID(userID)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		bytes, err := json.Marshal(friendIDList)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		return string(bytes), nil
	}
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "userID", userID, "friendIDList", friendIDList)
	}()
	friendIDListStr, err := db.DB.Rc.Fetch(friendRelationCache+userID, time.Second*30*60, getFriendIDList)
	if err != nil {
		return nil, utils.Wrap(err, "")
	}
	err = json.Unmarshal([]byte(friendIDListStr), &friendIDList)
	return friendIDList, utils.Wrap(err, "")
}

func DelFriendIDListFromCache(ctx context.Context, userID string) (err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "userID", userID)
	}()
	return db.DB.Rc.TagAsDeleted(friendRelationCache + userID)
}

func GetBlackListFromCache(ctx context.Context, userID string) (blackIDs []string, err error) {
	getBlackIDList := func() (string, error) {
		blackIDs, err := relation.GetBlackIDListByUserID(userID)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		bytes, err := json.Marshal(blackIDs)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		return string(bytes), nil
	}
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "userID", userID, "blackIDList", blackIDs)
	}()
	blackIDListStr, err := db.DB.Rc.Fetch(blackListCache+userID, time.Second*30*60, getBlackIDList)
	if err != nil {
		return nil, utils.Wrap(err, "")
	}
	err = json.Unmarshal([]byte(blackIDListStr), &blackIDs)
	return blackIDs, utils.Wrap(err, "")
}

func DelBlackIDListFromCache(ctx context.Context, userID string) (err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "ctx", ctx)
	}()
	return db.DB.Rc.TagAsDeleted(blackListCache + userID)
}

func GetJoinedGroupIDListFromCache(ctx context.Context, userID string) (joinedGroupList []string, err error) {
	getJoinedGroupIDList := func() (string, error) {
		joinedGroupList, err := relation.GetJoinedGroupIDListByUserID(userID)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		bytes, err := json.Marshal(joinedGroupList)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		return string(bytes), nil
	}
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "userID", userID, "joinedGroupList", joinedGroupList)
	}()
	joinedGroupIDListStr, err := db.DB.Rc.Fetch(joinedGroupListCache+userID, time.Second*30*60, getJoinedGroupIDList)
	if err != nil {
		return nil, utils.Wrap(err, "")
	}
	err = json.Unmarshal([]byte(joinedGroupIDListStr), &joinedGroupList)
	return joinedGroupList, utils.Wrap(err, "")
}

func DelJoinedGroupIDListFromCache(ctx context.Context, userID string) (err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "userID", userID)
	}()
	return db.DB.Rc.TagAsDeleted(joinedGroupListCache + userID)
}

func GetGroupMemberIDListFromCache(ctx context.Context, groupID string) (groupMemberIDList []string, err error) {
	f := func() (string, error) {
		groupInfo, err := GetGroupInfoFromCache(ctx, groupID)
		if err != nil {
			return "", utils.Wrap(err, "GetGroupInfoFromCache failed")
		}
		var groupMemberIDList []string
		if groupInfo.GroupType == constant.SuperGroup {
			superGroup, err := db.DB.GetSuperGroup(groupID)
			if err != nil {
				return "", utils.Wrap(err, "")
			}
			groupMemberIDList = superGroup.MemberIDList
		} else {
			groupMemberIDList, err = relation.GetGroupMemberIDListByGroupID(groupID)
			if err != nil {
				return "", utils.Wrap(err, "")
			}
		}
		bytes, err := json.Marshal(groupMemberIDList)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		return string(bytes), nil
	}
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "groupID", groupID, "groupMemberIDList", groupMemberIDList)
	}()
	groupIDListStr, err := db.DB.Rc.Fetch(groupCache+groupID, time.Second*30*60, f)
	if err != nil {
		return nil, utils.Wrap(err, "")
	}
	err = json.Unmarshal([]byte(groupIDListStr), &groupMemberIDList)
	return groupMemberIDList, utils.Wrap(err, "")
}

func DelGroupMemberIDListFromCache(ctx context.Context, groupID string) (err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "groupID", groupID)
	}()
	return db.DB.Rc.TagAsDeleted(groupCache + groupID)
}

func GetUserInfoFromCache(ctx context.Context, userID string) (userInfo *relation.User, err error) {
	getUserInfo := func() (string, error) {
		userInfo, err := relation.GetUserByUserID(userID)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		bytes, err := json.Marshal(userInfo)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		return string(bytes), nil
	}
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "userID", userID, "userInfo", *userInfo)
	}()
	userInfoStr, err := db.DB.Rc.Fetch(userInfoCache+userID, time.Second*30*60, getUserInfo)
	if err != nil {
		return nil, utils.Wrap(err, "")
	}
	userInfo = &relation.User{}
	err = json.Unmarshal([]byte(userInfoStr), userInfo)
	return userInfo, utils.Wrap(err, "")
}

func GetUserInfoFromCacheBatch(ctx context.Context, userIDs []string) ([]*relation.User, error) {
	var users []*relation.User
	for _, userID := range userIDs {
		user, err := GetUserInfoFromCache(ctx, userID)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func DelUserInfoFromCache(ctx context.Context, userID string) (err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "userID", userID)
	}()
	return db.DB.Rc.TagAsDeleted(userInfoCache + userID)
}

func GetGroupMemberInfoFromCache(ctx context.Context, groupID, userID string) (groupMember *relation.GroupMember, err error) {
	getGroupMemberInfo := func() (string, error) {
		groupMemberInfo, err := relation.GetGroupMemberInfoByGroupIDAndUserID(groupID, userID)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		bytes, err := json.Marshal(groupMemberInfo)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		return string(bytes), nil
	}
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "groupID", groupID, "userID", userID, "groupMember", *groupMember)
	}()
	groupMemberInfoStr, err := db.DB.Rc.Fetch(groupMemberInfoCache+groupID+"-"+userID, time.Second*30*60, getGroupMemberInfo)
	if err != nil {
		return nil, utils.Wrap(err, "")
	}
	groupMember = &relation.GroupMember{}
	err = json.Unmarshal([]byte(groupMemberInfoStr), groupMember)
	return groupMember, utils.Wrap(err, "")
}

func DelGroupMemberInfoFromCache(ctx context.Context, groupID, userID string) (err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "groupID", groupID, "userID", userID)
	}()
	return db.DB.Rc.TagAsDeleted(groupMemberInfoCache + groupID + "-" + userID)
}

func GetGroupMembersInfoFromCache(ctx context.Context, count, offset int32, groupID string) (groupMembers []*relation.GroupMember, err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "count", count, "offset", offset, "groupID", groupID, "groupMember", groupMembers)
	}()
	groupMemberIDList, err := GetGroupMemberIDListFromCache(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if count < 0 || offset < 0 {
		return nil, nil
	}
	var groupMemberList []*relation.GroupMember
	var start, stop int32
	start = offset
	stop = offset + count
	l := int32(len(groupMemberIDList))
	if start > stop {
		return nil, nil
	}
	if start >= l {
		return nil, nil
	}
	if count != 0 {
		if stop >= l {
			stop = l
		}
		groupMemberIDList = groupMemberIDList[start:stop]
	} else {
		if l < 1000 {
			stop = l
		} else {
			stop = 1000
		}
		groupMemberIDList = groupMemberIDList[start:stop]
	}
	//log.NewDebug("", utils.GetSelfFuncName(), "ID list: ", groupMemberIDList)
	for _, userID := range groupMemberIDList {
		groupMember, err := GetGroupMemberInfoFromCache(ctx, groupID, userID)
		if err != nil {
			log.NewError("", utils.GetSelfFuncName(), err.Error(), groupID, userID)
			continue
		}
		groupMembers = append(groupMembers, groupMember)
	}
	return groupMemberList, nil
}

func GetAllGroupMembersInfoFromCache(ctx context.Context, groupID string) (groupMembers []*relation.GroupMember, err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "groupID", groupID, "groupMembers", groupMembers)
	}()
	getGroupMemberInfo := func() (string, error) {
		groupMembers, err := relation.GetGroupMemberListByGroupID(groupID)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		bytes, err := json.Marshal(groupMembers)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		return string(bytes), nil
	}
	groupMembersStr, err := db.DB.Rc.Fetch(groupAllMemberInfoCache+groupID, time.Second*30*60, getGroupMemberInfo)
	if err != nil {
		return nil, utils.Wrap(err, "")
	}
	err = json.Unmarshal([]byte(groupMembersStr), &groupMembers)
	return groupMembers, utils.Wrap(err, "")
}

func DelAllGroupMembersInfoFromCache(ctx context.Context, groupID string) (err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "groupID", groupID)
	}()
	return db.DB.Rc.TagAsDeleted(groupAllMemberInfoCache + groupID)
}

//func GetGroupInfoFromCache(ctx context.Context, groupID string) (groupInfo *mysql.Group, err error) {
//	getGroupInfo := func() (string, error) {
//		groupInfo, err := mysql.GetGroupInfoByGroupID(groupID)
//		if err != nil {
//			return "", utils.Wrap(err, "")
//		}
//		bytes, err := json.Marshal(groupInfo)
//		if err != nil {
//			return "", utils.Wrap(err, "")
//		}
//		return string(bytes), nil
//	}
//	groupInfo = &mysql.Group{}
//	defer func() {
//		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "groupID", groupID, "groupInfo", groupInfo)
//	}()
//	groupInfoStr, err := db.DB.Rc.Fetch(groupInfoCache+groupID, time.Second*30*60, getGroupInfo)
//	if err != nil {
//		return nil, utils.Wrap(err, "")
//	}
//	err = json.Unmarshal([]byte(groupInfoStr), groupInfo)
//	return groupInfo, utils.Wrap(err, "")
//}
//
//func DelGroupInfoFromCache(ctx context.Context, groupID string) (err error) {
//	defer func() {
//		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "groupID", groupID)
//	}()
//	return db.DB.Rc.TagAsDeleted(groupInfoCache + groupID)
//}

func GetAllFriendsInfoFromCache(ctx context.Context, userID string) (friends []*relation.Friend, err error) {
	getAllFriendInfo := func() (string, error) {
		friendInfoList, err := relation.GetFriendListByUserID(userID)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		bytes, err := json.Marshal(friendInfoList)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		return string(bytes), nil
	}
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "userID", userID, "friends", friends)
	}()
	allFriendInfoStr, err := db.DB.Rc.Fetch(allFriendInfoCache+userID, time.Second*30*60, getAllFriendInfo)
	if err != nil {
		return nil, utils.Wrap(err, "")
	}
	err = json.Unmarshal([]byte(allFriendInfoStr), &friends)
	return friends, utils.Wrap(err, "")
}

func DelAllFriendsInfoFromCache(ctx context.Context, userID string) (err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "userID", userID)
	}()
	return db.DB.Rc.TagAsDeleted(allFriendInfoCache + userID)
}

func GetJoinedSuperGroupListFromCache(ctx context.Context, userID string) (joinedSuperGroupIDs []string, err error) {
	getJoinedSuperGroupIDList := func() (string, error) {
		userToSuperGroup, err := db.DB.GetSuperGroupByUserID(userID)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		bytes, err := json.Marshal(userToSuperGroup.GroupIDList)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		return string(bytes), nil
	}
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "userID", userID, "joinedSuperGroupIDs", joinedSuperGroupIDs)
	}()
	joinedSuperGroupListStr, err := db.DB.Rc.Fetch(joinedSuperGroupListCache+userID, time.Second*30*60, getJoinedSuperGroupIDList)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(joinedSuperGroupListStr), &joinedSuperGroupIDs)
	return joinedSuperGroupIDs, utils.Wrap(err, "")
}

func DelJoinedSuperGroupIDListFromCache(ctx context.Context, userID string) (err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "userID", userID)
	}()
	return db.DB.Rc.TagAsDeleted(joinedSuperGroupListCache + userID)
}

func GetGroupMemberListHashFromCache(ctx context.Context, groupID string) (hashCodeUint64 uint64, err error) {
	generateHash := func() (string, error) {
		groupInfo, err := GetGroupInfoFromCache(ctx, groupID)
		if err != nil {
			return "0", utils.Wrap(err, "GetGroupInfoFromCache failed")
		}
		if groupInfo.Status == constant.GroupStatusDismissed {
			return "0", nil
		}
		groupMemberIDList, err := GetGroupMemberIDListFromCache(ctx, groupID)
		if err != nil {
			return "", utils.Wrap(err, "GetGroupMemberIDListFromCache failed")
		}
		sort.Strings(groupMemberIDList)
		var all string
		for _, v := range groupMemberIDList {
			all += v
		}
		bi := big.NewInt(0)
		bi.SetString(utils.Md5(all)[0:8], 16)
		return strconv.Itoa(int(bi.Uint64())), nil
	}
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "groupID", groupID, "hashCodeUint64", hashCodeUint64)
	}()
	hashCodeStr, err := db.DB.Rc.Fetch(groupMemberListHashCache+groupID, time.Second*30*60, generateHash)
	if err != nil {
		return 0, utils.Wrap(err, "fetch failed")
	}
	hashCode, err := strconv.Atoi(hashCodeStr)
	return uint64(hashCode), err
}

func DelGroupMemberListHashFromCache(ctx context.Context, groupID string) (err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "groupID", groupID)
	}()
	return db.DB.Rc.TagAsDeleted(groupMemberListHashCache + groupID)
}

func GetGroupMemberNumFromCache(ctx context.Context, groupID string) (num int, err error) {
	getGroupMemberNum := func() (string, error) {
		num, err := relation.GetGroupMemberNumByGroupID(groupID)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		return strconv.Itoa(int(num)), nil
	}
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "groupID", groupID, "num", num)
	}()
	groupMember, err := db.DB.Rc.Fetch(groupMemberNumCache+groupID, time.Second*30*60, getGroupMemberNum)
	if err != nil {
		return 0, utils.Wrap(err, "")
	}
	return strconv.Atoi(groupMember)
}

func DelGroupMemberNumFromCache(ctx context.Context, groupID string) (err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "groupID", groupID)
	}()
	return db.DB.Rc.TagAsDeleted(groupMemberNumCache + groupID)
}

func GetUserConversationIDListFromCache(ctx context.Context, userID string) (conversationIDs []string, err error) {
	getConversationIDList := func() (string, error) {
		conversationIDList, err := relation.GetConversationIDListByUserID(userID)
		if err != nil {
			return "", utils.Wrap(err, "getConversationIDList failed")
		}
		log.NewDebug("", utils.GetSelfFuncName(), conversationIDList)
		bytes, err := json.Marshal(conversationIDList)
		if err != nil {
			return "", utils.Wrap(err, "")
		}
		return string(bytes), nil
	}
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "userID", userID, "conversationIDs", conversationIDs)
	}()
	conversationIDListStr, err := db.DB.Rc.Fetch(conversationIDListCache+userID, time.Second*30*60, getConversationIDList)
	err = json.Unmarshal([]byte(conversationIDListStr), &conversationIDs)
	if err != nil {
		return nil, utils.Wrap(err, "")
	}
	return conversationIDs, nil
}

func DelUserConversationIDListFromCache(ctx context.Context, userID string) (err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "userID", userID)
	}()
	return utils.Wrap(db.DB.Rc.TagAsDeleted(conversationIDListCache+userID), "DelUserConversationIDListFromCache err")
}

func GetConversationFromCache(ctx context.Context, ownerUserID, conversationID string) (conversation *relation.Conversation, err error) {
	getConversation := func() (string, error) {
		conversation, err := relation.GetConversation(ownerUserID, conversationID)
		if err != nil {
			return "", utils.Wrap(err, "get failed")
		}
		bytes, err := json.Marshal(conversation)
		if err != nil {
			return "", utils.Wrap(err, "Marshal failed")
		}
		return string(bytes), nil
	}
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "ownerUserID", ownerUserID, "conversationID", conversationID, "conversation", *conversation)
	}()
	conversationStr, err := db.DB.Rc.Fetch(conversationCache+ownerUserID+":"+conversationID, time.Second*30*60, getConversation)
	if err != nil {
		return nil, utils.Wrap(err, "Fetch failed")
	}
	conversation = &relation.Conversation{}
	err = json.Unmarshal([]byte(conversationStr), &conversation)
	return conversation, utils.Wrap(err, "Unmarshal failed")
}

func GetConversationsFromCache(ctx context.Context, ownerUserID string, conversationIDs []string) (conversations []relation.Conversation, err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "ownerUserID", ownerUserID, "conversationIDs", conversationIDs, "conversations", conversations)
	}()
	for _, conversationID := range conversationIDs {
		conversation, err := GetConversationFromCache(ctx, ownerUserID, conversationID)
		if err != nil {
			return nil, utils.Wrap(err, "GetConversationFromCache failed")
		}
		conversations = append(conversations, *conversation)
	}
	return conversations, nil
}

func GetUserAllConversationList(ctx context.Context, ownerUserID string) (conversations []relation.Conversation, err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "ownerUserID", ownerUserID, "conversations", conversations)
	}()
	IDList, err := GetUserConversationIDListFromCache(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	var conversationList []relation.Conversation
	log.NewDebug("", utils.GetSelfFuncName(), IDList)
	for _, conversationID := range IDList {
		conversation, err := GetConversationFromCache(ctx, ownerUserID, conversationID)
		if err != nil {
			return nil, utils.Wrap(err, "GetConversationFromCache failed")
		}
		conversationList = append(conversationList, *conversation)
	}
	return conversationList, nil
}

func DelConversationFromCache(ctx context.Context, ownerUserID, conversationID string) (err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "ownerUserID", ownerUserID, "conversationID", conversationID)
	}()
	return utils.Wrap(db.DB.Rc.TagAsDeleted(conversationCache+ownerUserID+":"+conversationID), "DelConversationFromCache err")
}

func GetExtendMsg(ctx context.Context, sourceID string, sessionType int32, clientMsgID string, firstModifyTime int64) (extendMsg *mongoDB.ExtendMsg, err error) {
	getExtendMsg := func() (string, error) {
		extendMsg, err := db.DB.GetExtendMsg(sourceID, sessionType, clientMsgID, firstModifyTime)
		if err != nil {
			return "", utils.Wrap(err, "GetExtendMsgList failed")
		}
		bytes, err := json.Marshal(extendMsg)
		if err != nil {
			return "", utils.Wrap(err, "Marshal failed")
		}
		return string(bytes), nil
	}
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "sourceID", sourceID, "sessionType",
			sessionType, "clientMsgID", clientMsgID, "firstModifyTime", firstModifyTime, "extendMsg", extendMsg)
	}()
	extendMsgStr, err := db.DB.Rc.Fetch(extendMsgCache+clientMsgID, time.Second*30*60, getExtendMsg)
	if err != nil {
		return nil, utils.Wrap(err, "Fetch failed")
	}
	extendMsg = &mongoDB.ExtendMsg{}
	err = json.Unmarshal([]byte(extendMsgStr), extendMsg)
	return extendMsg, utils.Wrap(err, "Unmarshal failed")
}

func DelExtendMsg(ctx context.Context, clientMsgID string) (err error) {
	defer func() {
		tracelog.SetCtxDebug(ctx, utils.GetFuncName(1), err, "clientMsgID", clientMsgID)
	}()
	return utils.Wrap(db.DB.Rc.TagAsDeleted(extendMsgCache+clientMsgID), "DelExtendMsg err")
}
