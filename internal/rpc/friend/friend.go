package friend

import (
	"Open_IM/internal/common/check"
	"Open_IM/internal/common/convert"
	"Open_IM/internal/common/notification"
	"Open_IM/pkg/common/constant"
	"Open_IM/pkg/common/db/controller"
	"Open_IM/pkg/common/db/relation"
	relationTb "Open_IM/pkg/common/db/table/relation"
	"Open_IM/pkg/common/tokenverify"
	"Open_IM/pkg/common/tracelog"
	discoveryRegistry "Open_IM/pkg/discoveryregistry"
	pbfriend "Open_IM/pkg/proto/friend"
	"Open_IM/pkg/utils"
	"context"
	"github.com/OpenIMSDK/openKeeper"
	"google.golang.org/grpc"
)

type friendServer struct {
	controller.FriendInterface
	controller.BlackInterface
	notification   *notification.Check
	userCheck      *check.UserCheck
	RegisterCenter discoveryRegistry.SvcDiscoveryRegistry
}

func Start(client *openKeeper.ZkClient, server *grpc.Server) error {
	mysql, err := relation.NewGormDB()
	if err != nil {
		return err
	}
	if err := mysql.AutoMigrate(&relationTb.FriendModel{}, &relationTb.FriendRequestModel{}, &relationTb.BlackModel{}); err != nil {
		return err
	}
	pbfriend.RegisterFriendServer(server, &friendServer{
		FriendInterface: controller.NewFriendController(mysql),
		BlackInterface:  controller.NewBlackController(mysql),
		notification:    notification.NewCheck(client),
		userCheck:       check.NewUserCheck(client),
		RegisterCenter:  client,
	})
	return nil
}

// ok
func (s *friendServer) ApplyToAddFriend(ctx context.Context, req *pbfriend.ApplyToAddFriendReq) (resp *pbfriend.ApplyToAddFriendResp, err error) {
	resp = &pbfriend.ApplyToAddFriendResp{}
	if err := tokenverify.CheckAccessV3(ctx, req.FromUserID); err != nil {
		return nil, err
	}
	if err := callbackBeforeAddFriendV1(ctx, req); err != nil {
		return nil, err
	}
	if req.ToUserID == req.FromUserID {
		return nil, constant.ErrCanNotAddYourself.Wrap()
	}
	if _, err := s.userCheck.GetUsersInfoMap(ctx, []string{req.ToUserID, req.FromUserID}, true); err != nil {
		return nil, err
	}
	in1, in2, err := s.FriendInterface.CheckIn(ctx, req.FromUserID, req.ToUserID)
	if err != nil {
		return nil, err
	}
	if in1 && in2 {
		return nil, constant.ErrRelationshipAlready.Wrap()
	}
	if err = s.FriendInterface.AddFriendRequest(ctx, req.FromUserID, req.ToUserID, req.ReqMsg, req.Ex); err != nil {
		return nil, err
	}
	s.notification.FriendApplicationAddNotification(ctx, req)
	return resp, nil
}

// ok
func (s *friendServer) ImportFriends(ctx context.Context, req *pbfriend.ImportFriendReq) (resp *pbfriend.ImportFriendResp, err error) {
	resp = &pbfriend.ImportFriendResp{}
	if err := tokenverify.CheckAdmin(ctx); err != nil {
		return nil, err
	}
	if _, err := s.userCheck.GetUsersInfos(ctx, append([]string{req.OwnerUserID}, req.FriendUserIDs...), true); err != nil {
		return nil, err
	}

	if utils.Contain(req.OwnerUserID, req.FriendUserIDs...) {
		return nil, constant.ErrCanNotAddYourself.Wrap()
	}
	if utils.Duplicate(req.FriendUserIDs) {
		return nil, constant.ErrArgs.Wrap("friend userID repeated")
	}

	if err := s.FriendInterface.BecomeFriends(ctx, req.OwnerUserID, req.FriendUserIDs, constant.BecomeFriendByImport, tracelog.GetOpUserID(ctx)); err != nil {
		return nil, err
	}
	return resp, nil
}

// ok
func (s *friendServer) RespondFriendApply(ctx context.Context, req *pbfriend.RespondFriendApplyReq) (resp *pbfriend.RespondFriendApplyResp, err error) {
	resp = &pbfriend.RespondFriendApplyResp{}
	if err := s.userCheck.Access(ctx, req.ToUserID); err != nil {
		return nil, err
	}
	friendRequest := relationTb.FriendRequestModel{FromUserID: req.FromUserID, ToUserID: req.ToUserID, HandleMsg: req.HandleMsg, HandleResult: req.HandleResult}
	if req.HandleResult == constant.FriendResponseAgree {
		err := s.AgreeFriendRequest(ctx, &friendRequest)
		if err != nil {
			return nil, err
		}
		s.notification.FriendApplicationAgreedNotification(ctx, req)
		return resp, nil
	}
	if req.HandleResult == constant.FriendResponseRefuse {
		err := s.RefuseFriendRequest(ctx, &friendRequest)
		if err != nil {
			return nil, err
		}
		s.notification.FriendApplicationRefusedNotification(ctx, req)
		return resp, nil
	}
	return nil, constant.ErrArgs.Wrap("req.HandleResult != -1/1")
}

// ok
func (s *friendServer) DeleteFriend(ctx context.Context, req *pbfriend.DeleteFriendReq) (resp *pbfriend.DeleteFriendResp, err error) {
	resp = &pbfriend.DeleteFriendResp{}
	if err := s.userCheck.Access(ctx, req.OwnerUserID); err != nil {
		return nil, err
	}
	_, err = s.FindFriendsWithError(ctx, req.OwnerUserID, []string{req.FriendUserID})
	if err != nil {
		return nil, err
	}
	if err := s.FriendInterface.Delete(ctx, req.OwnerUserID, []string{req.FriendUserID}); err != nil {
		return nil, err
	}
	s.notification.FriendDeletedNotification(ctx, req)
	return resp, nil
}

// ok
func (s *friendServer) SetFriendRemark(ctx context.Context, req *pbfriend.SetFriendRemarkReq) (resp *pbfriend.SetFriendRemarkResp, err error) {
	resp = &pbfriend.SetFriendRemarkResp{}
	if err := s.userCheck.Access(ctx, req.OwnerUserID); err != nil {
		return nil, err
	}
	_, err = s.FindFriendsWithError(ctx, req.OwnerUserID, []string{req.FriendUserID})
	if err != nil {
		return nil, err
	}
	if err := s.FriendInterface.UpdateRemark(ctx, req.OwnerUserID, req.FriendUserID, req.Remark); err != nil {
		return nil, err
	}
	s.notification.FriendRemarkSetNotification(ctx, req.OwnerUserID, req.FriendUserID)
	return resp, nil
}

// ok
func (s *friendServer) GetDesignatedFriends(ctx context.Context, req *pbfriend.GetDesignatedFriendsReq) (resp *pbfriend.GetDesignatedFriendsResp, err error) {
	resp = &pbfriend.GetDesignatedFriendsResp{}
	if err := s.userCheck.Access(ctx, req.UserID); err != nil {
		return nil, err
	}
	friends, total, err := s.FriendInterface.PageOwnerFriends(ctx, req.UserID, req.Pagination.PageNumber, req.Pagination.ShowNumber)
	if err != nil {
		return nil, err
	}
	resp.FriendsInfo, err = (*convert.NewDBFriend(nil, s.RegisterCenter)).DB2PB(ctx, friends)
	if err != nil {
		return nil, err
	}
	resp.Total = int32(total)
	return resp, nil
}

// ok 获取接收到的好友申请（即别人主动申请的）
func (s *friendServer) GetPaginationFriendsApplyTo(ctx context.Context, req *pbfriend.GetPaginationFriendsApplyToReq) (resp *pbfriend.GetPaginationFriendsApplyToResp, err error) {
	resp = &pbfriend.GetPaginationFriendsApplyToResp{}
	if err := s.userCheck.Access(ctx, req.UserID); err != nil {
		return nil, err
	}
	friendRequests, total, err := s.FriendInterface.PageFriendRequestToMe(ctx, req.UserID, req.Pagination.PageNumber, req.Pagination.ShowNumber)
	if err != nil {
		return nil, err
	}
	resp.FriendRequests, err = (*convert.NewDBFriendRequest(nil, s.RegisterCenter)).DB2PB(ctx, friendRequests)
	if err != nil {
		return nil, err
	}
	resp.Total = int32(total)
	return resp, nil
}

// ok 获取主动发出去的好友申请列表
func (s *friendServer) GetPaginationFriendsApplyFrom(ctx context.Context, req *pbfriend.GetPaginationFriendsApplyFromReq) (resp *pbfriend.GetPaginationFriendsApplyFromResp, err error) {
	resp = &pbfriend.GetPaginationFriendsApplyFromResp{}
	if err := s.userCheck.Access(ctx, req.UserID); err != nil {
		return nil, err
	}
	friendRequests, total, err := s.FriendInterface.PageFriendRequestFromMe(ctx, req.UserID, req.Pagination.PageNumber, req.Pagination.ShowNumber)
	if err != nil {
		return nil, err
	}
	resp.FriendRequests, err = (*convert.NewDBFriendRequest(nil, s.RegisterCenter)).DB2PB(ctx, friendRequests)
	if err != nil {
		return nil, err
	}
	resp.Total = int32(total)
	return resp, nil
}

// ok
func (s *friendServer) IsFriend(ctx context.Context, req *pbfriend.IsFriendReq) (resp *pbfriend.IsFriendResp, err error) {
	resp = &pbfriend.IsFriendResp{}
	resp.InUser1Friends, resp.InUser2Friends, err = s.FriendInterface.CheckIn(ctx, req.UserID1, req.UserID2)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ok
func (s *friendServer) GetPaginationFriends(ctx context.Context, req *pbfriend.GetPaginationFriendsReq) (resp *pbfriend.GetPaginationFriendsResp, err error) {
	resp = &pbfriend.GetPaginationFriendsResp{}
	if utils.Duplicate(req.FriendUserIDs) {
		return nil, constant.ErrArgs.Wrap("friend userID repeated")
	}
	friends, err := s.FriendInterface.FindFriendsWithError(ctx, req.OwnerUserID, req.FriendUserIDs)
	if err != nil {
		return nil, err
	}
	if resp.FriendsInfo, err = (*convert.NewDBFriend(nil, s.RegisterCenter)).DB2PB(ctx, friends); err != nil {
		return nil, err
	}
	return resp, nil
}
