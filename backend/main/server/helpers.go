package server

import (
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/DapperCollectives/CAST/backend/main/models"
	"github.com/DapperCollectives/CAST/backend/main/shared"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v4"
	"github.com/rs/zerolog/log"
	"github.com/thoas/go-funk"
)

type Helpers struct {
	A *App
}

func (h *Helpers) Initialize(app *App) {
	h.A = app
}

func (h *Helpers) uploadFile(r *http.Request) (interface{}, error) {
	file, handler, err := r.FormFile("file")
	if err != nil {
		log.Error().Err(err).Msg("FormFile Retrieval Error.")
		return nil, err
	}
	defer file.Close()

	// ensure mime type is allowed
	mime := handler.Header.Get("Content-Type")
	if !funk.Contains(allowedFileTypes, mime) {
		msg := fmt.Sprintf("Uploaded file type of '%s' is not allowed.", mime)
		log.Error().Msg(msg)
		return nil, errors.New(msg)
	}

	pin, err := h.A.IpfsClient.PinFile(file, handler.Filename)
	if err != nil {
		log.Error().Err(err).Msg("Error pinning file to IPFS.")
		return nil, err
	}

	resp := struct {
		Cid string `json:"cid"`
	}{
		Cid: pin.IpfsHash,
	}

	return resp, nil
}

func (h *Helpers) fetchProposal(vars map[string]string, query string) (models.Proposal, error) {
	proposalId, err := strconv.Atoi(vars[query])
	if err != nil {
		msg := fmt.Sprintf("Invalid proposalId: %s", vars["proposalId"])
		log.Error().Err(err).Msg(msg)
		return models.Proposal{}, errors.New(msg)
	}

	p := models.Proposal{ID: proposalId}

	if err := p.GetProposalById(h.A.DB); err != nil {
		switch err.Error() {
		case pgx.ErrNoRows.Error():
			msg := fmt.Sprintf("Proposal with ID %d not found.", proposalId)
			return models.Proposal{}, errors.New(msg)
		default:
			return models.Proposal{}, err
		}
	}

	return p, nil
}

func (h *Helpers) useStrategyTally(
	p models.Proposal,
	v []*models.VoteWithBalance,
) (models.ProposalResults, error) {

	s := h.initStrategy(*p.Strategy)
	if s == nil {
		return models.ProposalResults{}, errors.New("Strategy not found.")
	}

	proposalInitialized := models.NewProposalResults(p.ID, p.Choices)
	results, err := s.TallyVotes(v, proposalInitialized, &p)
	if err != nil {
		return models.ProposalResults{}, err
	}

	return results, nil
}

func (h *Helpers) useStrategyGetVotes(
	p models.Proposal,
	v []*models.VoteWithBalance,
) ([]*models.VoteWithBalance, error) {

	s := h.initStrategy(*p.Strategy)
	if s == nil {
		return nil, errors.New("Strategy not found.")
	}

	votesWithWeights, err := s.GetVotes(v, &p)
	if err != nil {
		return nil, err
	}

	return votesWithWeights, nil
}

func (h *Helpers) useStrategyGetVoteWeight(
	p models.Proposal,
	v *models.VoteWithBalance,
) (float64, error) {
	s := strategyMap[*p.Strategy]
	if s == nil {
		return 0, errors.New("Strategy not found.")
	}

	weight, err := s.GetVoteWeightForBalance(v, &p)
	if err != nil {
		return 0, err
	}
	return weight, nil
}

func (h *Helpers) getPaginatedVotes(
	r *http.Request,
	p models.Proposal,
) (
	[]*models.VoteWithBalance,
	shared.OrderedPageParams,
	error,
) {

	start, count, order := getOrderedPageParams(
		r.FormValue("start"),
		r.FormValue("count"),
		r.FormValue("order"),
		25,
	)

	votes, totalRecords, err := models.GetVotesForProposal(
		h.A.DB,
		start,
		count,
		order,
		p.ID,
		*p.Strategy,
	)
	if err != nil {
		return nil, shared.OrderedPageParams{}, err
	}

	ordered := shared.OrderedPageParams{
		Start:        start,
		Count:        count,
		Order:        order,
		TotalRecords: totalRecords,
	}

	return votes, ordered, nil
}

func (h *Helpers) processVote(addr string, p models.Proposal) (*models.VoteWithBalance, error) {
	vote, err := h.fetchVote(addr, p.ID)
	if err != nil {
		return nil, err
	}

	weight, err := h.useStrategyGetVoteWeight(p, vote)
	if err != nil {
		return nil, err
	}

	vote.Weight = &weight
	return vote, err
}

func (h *Helpers) fetchVote(addr string, id int) (*models.VoteWithBalance, error) {
	voteWithBalance := &models.VoteWithBalance{
		Vote: models.Vote{
			Addr:        addr,
			Proposal_id: id,
		}}

	if err := voteWithBalance.GetVote(h.A.DB); err != nil {
		switch err.Error() {
		case pgx.ErrNoRows.Error():
			msg := fmt.Sprintf("Vote not found.")
			return nil, errors.New(msg)
		default:
			return nil, err
		}
	}
	return voteWithBalance, nil
}

func (h *Helpers) processVotes(
	addr string,
	ids []int,
	order shared.OrderedPageParams,
) (
	[]*models.VoteWithBalance,
	shared.OrderedPageParams,
	error,
) {
	votes, totalRecords, err := models.GetVotesForAddress(
		h.A.DB,
		order.Start,
		order.Count,
		addr,
		&ids,
	)
	if err != nil {
		log.Error().Err(err).Msg("Error getting votes for address.")
		return nil, order, err
	}

	var votesWithBalances []*models.VoteWithBalance

	for _, vote := range votes {

		proposal := models.Proposal{ID: vote.Proposal_id}
		if err := proposal.GetProposalById(h.A.DB); err != nil {
			switch err.Error() {
			case pgx.ErrNoRows.Error():
				msg := fmt.Sprintf("Proposal with ID %d not found.", vote.Proposal_id)
				return nil, order, errors.New(msg)
			default:
				return nil, order, err
			}
		}

		s := strategyMap[*proposal.Strategy]
		if s == nil {
			return nil, order, errors.New("Strategy not found.")
		}

		weight, err := s.GetVoteWeightForBalance(vote, &proposal)
		if err != nil {
			return nil, order, err
		}

		vote.Weight = &weight
		votesWithBalances = append(votesWithBalances, vote)
	}

	order.TotalRecords = totalRecords

	return votesWithBalances, order, nil
}

func (h *Helpers) createVote(r *http.Request, p models.Proposal) (*models.VoteWithBalance, error) {
	var payload models.CreateVotePayload
	if err := validatePayload(r.Body, &payload); err != nil {
		log.Error().Err(err).Msg("Invalid request payload.")
		return nil, err
	}

	var v = payload.Vote

	v.Proposal_id = p.ID

	// validate user hasn't already voted
	existingVote := models.Vote{Proposal_id: v.Proposal_id, Addr: v.Addr}
	if err := existingVote.GetVote(h.A.DB); err == nil {
		log.Error().Msgf("Address %s has already voted for proposal %d.", v.Addr, v.Proposal_id)
		return nil, errors.New("Address has already voted for this proposal.")
	}

	// check that proposal is live
	if os.Getenv("APP_ENV") != "DEV" {
		if !p.IsLive() {
			err := errors.New("User cannot vote on inactive proposal.")
			return nil, err
		}
	}

	if err := h.validateVote(p, payload); err != nil {
		return nil, err
	}

	v.Proposal_id = p.ID

	s := h.initStrategy(*p.Strategy)
	if s == nil {
		return nil, errors.New("Proposal strategy not found.")
	}

	emptyBalance := &models.Balance{
		Addr:        v.Addr,
		Proposal_id: p.ID,
	}
	if p.Block_height != nil {
		emptyBalance.BlockHeight = *p.Block_height
	}

	balance, err := s.FetchBalance(emptyBalance, &p)
	if err != nil {
		log.Error().Err(err).Msgf("Error fetching balance for address %v.", v.Addr)
		return nil, errors.New("Error fetching balance for address.")
	}

	vb := models.VoteWithBalance{
		Vote:                    v,
		PrimaryAccountBalance:   &balance.PrimaryAccountBalance,
		SecondaryAccountBalance: &balance.SecondaryAccountBalance,
		StakingBalance:          &balance.StakingBalance,
	}

	weight, err := s.GetVoteWeightForBalance(&vb, &p)
	if err != nil {
		msg := fmt.Sprintf("Error getting vote weight for address %s.", v.Addr)
		log.Error().Err(err).Msg(msg)
		return nil, errors.New(msg)
	}

	if err = p.ValidateBalance(weight); err != nil {
		msg := fmt.Sprintf("Account balance is too low to vote on this proposal.")
		log.Error().Err(err).Msg(msg)
		return nil, errors.New(msg)
	}

	// Include voucher in vote data when pinning
	ipfsVote := map[string]interface{}{
		"vote":    v,
		"voucher": payload.Voucher,
	}
	v.Cid, err = h.pinJSONToIpfs(ipfsVote)
	if err != nil {
		msg := fmt.Sprintf("Error pinning proposal to IPFS.")
		log.Error().Err(err).Msg(msg)
		return nil, errors.New(msg)
	}

	if err := v.CreateVote(h.A.DB); err != nil {
		msg := fmt.Sprintf("Error creating vote for address %s.", v.Addr)
		log.Error().Err(err).Msg(msg)
		return nil, errors.New(msg)
	}

	return &vb, nil
}

func (h *Helpers) validateVote(p models.Proposal, payload models.CreateVotePayload) error {
	var v = payload.Vote
	// validate the user is not on community's blocklist
	if err := h.validateBlocklist(v.Addr, p.Community_id); err != nil {
		log.Error().Err(err).Msgf(fmt.Sprintf("Address %v is on blocklist for community id %v.\n", v.Addr, p.Community_id))
		msg := fmt.Sprintf("Address %v is on blocklist for community id %v.", v.Addr, p.Community_id)
		return errors.New(msg)
	}

	// validate choice exists on proposal
	if err := v.ValidateChoice(p); err != nil {
		log.Error().Err(err)
		return err
	}

	// If voucher is present
	if payload.Voucher != nil {
		// Transaction Signature validation
		voucher := payload.Voucher
		authorizer := voucher.Authorizers[0]

		v.Composite_signatures = shared.GetUserCompositeSignatureFromVoucher(voucher)

		// Validate authorizer
		if authorizer != v.Addr || authorizer != (*v.Composite_signatures)[0].Addr {
			err := errors.New("authorizer address must match voter address and envelope signer")
			log.Error().Err(err)
			return err
		}

		voteMessageToValidate := fmt.Sprintf("%s:%s:%s",
			voucher.Arguments[0]["value"],
			voucher.Arguments[1]["value"],
			voucher.Arguments[2]["value"],
		)
		// validate proper message format
		//<proposalId>:<choice>:<timestamp>
		if err := models.ValidateVoteMessage(voteMessageToValidate, p); err != nil {
			log.Error().Err(err)
			return err
		}

		// re-build message & composite signatures for validation
		// set v.Message as the encoded message, rather than the colon(:) delimited message above.
		// we can do this because we can always recover the tx arguments that make up the
		// colon delimited message by decoding this rlp encoded message
		v.Message = shared.EncodeMessageFromVoucher(voucher)

		if err := h.validateTxSignature(v.Addr, v.Message, v.Composite_signatures); err != nil {
			return err
		}
	} else {
		// validate proper message format
		// hex decode before validating
		decodedMessage, _ := hex.DecodeString(v.Message)
		if err := models.ValidateVoteMessage(string(decodedMessage), p); err != nil {
			log.Error().Err(err)
			return err
		}
		if err := h.validateUserSignature(v.Addr, v.Message, v.Composite_signatures); err != nil {
			return err
		}
	}

	return nil
}

func (h *Helpers) fetchCommunity(id int) (models.Community, int, error) {
	community := models.Community{ID: id}

	if err := community.GetCommunity(h.A.DB); err != nil {
		log.Error().Err(err)
		switch err.Error() {
		case pgx.ErrNoRows.Error():
			return models.Community{}, http.StatusNotFound, errors.New("Community not found.")
		default:
			return models.Community{}, http.StatusInternalServerError, err
		}
	}

	return community, http.StatusOK, nil
}

func (h *Helpers) createProposal(communityId int, p models.Proposal) (models.Proposal, int, error) {
	if err := h.validateUser(p.Creator_addr, p.Timestamp, p.Composite_signatures); err != nil {
		return models.Proposal{}, http.StatusForbidden, err
	}

	community, httpStatus, err := h.fetchCommunity(communityId)
	if err != nil {
		return models.Proposal{}, httpStatus, err
	}

	strategy, err := models.MatchStrategyByProposal(*community.Strategies, *p.Strategy)
	if err != nil {
		errMsg := "Community does not have this strategy available."
		log.Error().Err(err).Msg(errMsg)
		return models.Proposal{}, http.StatusInternalServerError, errors.New(errMsg)

	}
	s := h.initStrategy(*strategy.Name)

	p.Min_balance = strategy.Contract.Threshold
	p.Max_weight = strategy.Contract.MaxWeight

	var snapshotResponse *shared.SnapshotResponse
	if s.RequiresSnapshot() {
		snapshotResponse, err = h.A.SnapshotClient.TakeSnapshot(strategy.Contract)
		if err != nil {
			errMsg := "Error taking snapshot."
			log.Error().Err(err).Msg(errMsg)
			return models.Proposal{}, http.StatusInternalServerError, errors.New(errMsg)
		}
		p.Block_height = &snapshotResponse.Data.BlockHeight
		p.Snapshot_status = &snapshotResponse.Data.Status
	}

	if *community.Only_authors_to_submit {
		if err := models.EnsureRoleForCommunity(h.A.DB, p.Creator_addr, communityId, "author"); err != nil {
			errMsg := fmt.Sprintf("Account %s is not an author for community %d.", p.Creator_addr, p.Community_id)
			log.Error().Err(err).Msg(errMsg)
			return models.Proposal{}, http.StatusForbidden, errors.New(errMsg)
		}
	} else {
		hasBalance, err := h.processTokenThreshold(p.Creator_addr, strategy)
		if err != nil {
			errMsg := "Error processing Token Threshold."
			log.Error().Err(err).Msg(errMsg)
			return models.Proposal{}, http.StatusForbidden, errors.New(errMsg)
		}

		if !hasBalance {
			errMsg := "Insufficient token balance to create proposal."
			log.Error().Err(err).Msg(errMsg)
			return models.Proposal{}, http.StatusForbidden, errors.New(errMsg)
		}
	}

	if err := h.processSnapshotStatus(&strategy, &p); err != nil {
		errMsg := "Error processing snapshot status."
		log.Error().Err(err).Msg(errMsg)
		return models.Proposal{}, http.StatusInternalServerError, errors.New(errMsg)
	}

	p.Cid, err = h.pinJSONToIpfs(p)
	if err != nil {
		log.Error().Err(err).Msg("IPFS error: " + err.Error())
		errMsg := "Error pinning JSON to IPFS."
		return models.Proposal{}, http.StatusInternalServerError, errors.New(errMsg)
	}

	// validate proposal fields
	validate := validator.New()
	vErr := validate.Struct(p)
	if vErr != nil {
		log.Error().Err(vErr)
		return models.Proposal{}, http.StatusBadRequest, errors.New("Invalid proposal.")
	}

	if os.Getenv("APP_ENV") == "PRODUCTION" {
		if strategy.Contract.Name != nil && p.Start_time.Before(time.Now().UTC().Add(time.Hour)) {
			p.Start_time = time.Now().UTC().Add(time.Hour)
		}
	}

	if err := p.CreateProposal(h.A.DB); err != nil {
		return models.Proposal{}, http.StatusInternalServerError, err
	}

	return p, http.StatusCreated, nil
}

func (h *Helpers) createCommunity(payload models.CreateCommunityRequestPayload) (models.Community, int, error) {
	c := payload.Community

	if err := h.validateUser(c.Creator_addr, c.Timestamp, c.Composite_signatures); err != nil {
		return models.Community{}, http.StatusForbidden, err
	}

	cid, err := h.pinJSONToIpfs(c)
	if err != nil {
		errMsg := "Error pinning JSON to IPFS."
		log.Error().Err(err).Msg(errMsg)
		return models.Community{}, http.StatusInternalServerError, errors.New(errMsg)
	}
	c.Cid = cid

	validate := validator.New()
	vErr := validate.Struct(c)
	if vErr != nil {
		errMsg := "Invalid community."
		log.Error().Err(vErr).Msg(errMsg)
		return models.Community{}, http.StatusBadRequest, errors.New(errMsg)
	}

	if err := c.CreateCommunity(h.A.DB); err != nil {
		errMsg := "Database error creating community."
		log.Error().Err(err).Msg(errMsg)
		return models.Community{}, http.StatusInternalServerError, errors.New(errMsg)
	}

	if err := models.GrantRolesToCommunityCreator(h.A.DB, c.Creator_addr, c.ID); err != nil {
		errMsg := "Database error adding community creator roles."
		log.Error().Err(err).Msg(errMsg)
		return models.Community{}, http.StatusInternalServerError, errors.New(errMsg)
	}

	if payload.Additional_admins != nil {
		for _, addr := range *payload.Additional_admins {
			if err := models.GrantAdminRolesToAddress(h.A.DB, c.ID, addr); err != nil {
				log.Error().Err(err)
				return models.Community{}, http.StatusInternalServerError, err
			}
		}
	}

	if payload.Additional_authors != nil {
		for _, addr := range *payload.Additional_authors {
			if err := models.GrantAuthorRolesToAddress(h.A.DB, c.ID, addr); err != nil {
				log.Error().Err(err)
				return models.Community{}, http.StatusInternalServerError, err
			}
		}
	}

	return c, http.StatusCreated, nil
}

func (h *Helpers) updateCommunity(id int, payload models.UpdateCommunityRequestPayload) (models.Community, int, error) {
	c, httpStatus, err := h.fetchCommunity(id)
	if err != nil {
		return models.Community{}, httpStatus, err
	}

	// validate is community creator
	// TODO: update to validating address is admin
	if err := c.CanUpdateCommunity(h.A.DB, payload.Signing_addr); err != nil {
		log.Error().Err(err)
		return models.Community{}, http.StatusForbidden, err
	}

	if err := h.validateUser(payload.Signing_addr, payload.Timestamp, payload.Composite_signatures); err != nil {
		log.Error().Err(err)
		return models.Community{}, http.StatusForbidden, err
	}

	if err := c.UpdateCommunity(h.A.DB, &payload); err != nil {
		log.Error().Err(err)
		return models.Community{}, http.StatusInternalServerError, err
	}

	c, httpStatus, err = h.fetchCommunity(id)
	if err != nil {
		return models.Community{}, httpStatus, err
	}

	return c, http.StatusOK, nil
}

func (h *Helpers) removeUserRole(payload models.CommunityUserPayload) (int, error) {
	if err := h.validateUser(payload.Signing_addr, payload.Timestamp, payload.Composite_signatures); err != nil {
		return http.StatusForbidden, err
	}

	if payload.User_type == "member" {
		if payload.Addr == payload.Signing_addr {
			// If a member is removing themselves, remove all their other roles as well
			userRoles, err := models.GetAllRolesForUserInCommunity(h.A.DB, payload.Addr, payload.Community_id)
			if err != nil {
				log.Error().Err(err)
				return http.StatusInternalServerError, err
			}
			for _, userRole := range userRoles {
				if err := userRole.Remove(h.A.DB); err != nil {
					log.Error().Err(err)
					return http.StatusInternalServerError, err
				}
			}
		} else {
			// validate someone else is not removing a "member" role
			CANNOT_REMOVE_MEMBER_ERR := errors.New("Cannot remove another member from a community.")
			log.Error().Err(CANNOT_REMOVE_MEMBER_ERR)
			return http.StatusForbidden, CANNOT_REMOVE_MEMBER_ERR
		}
	}

	u := payload.CommunityUser

	if payload.User_type == "admin" {
		// validate signer is admin
		var adminUser = models.CommunityUser{Addr: payload.Signing_addr, Community_id: payload.Community_id, User_type: "admin"}
		if err := adminUser.GetCommunityUser(h.A.DB); err != nil {
			USER_MUST_BE_ADMIN_ERR := errors.New("User must be community admin.")
			log.Error().Err(err).Msg("Database error.")
			log.Error().Err(USER_MUST_BE_ADMIN_ERR)
			return http.StatusForbidden, USER_MUST_BE_ADMIN_ERR
		}
		// If the admin role is being removed, remove author role as well
		author := models.CommunityUser{Addr: u.Addr, Community_id: u.Community_id, User_type: "author"}
		if err := author.Remove(h.A.DB); err != nil {
			return http.StatusInternalServerError, err
		}
		// remove admin role
		if err := u.Remove(h.A.DB); err != nil {
			return http.StatusInternalServerError, err
		}
		// Otherwise, just remove the specified user role
	} else if err := u.Remove(h.A.DB); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}

func (h *Helpers) createCommunityUser(payload models.CommunityUserPayload) (int, error) {
	// validate community_user payload fields
	validate := validator.New()
	vErr := validate.Struct(payload)
	if vErr != nil {
		errMsg := "Invalid community user."
		log.Error().Err(vErr).Msg(errMsg)
		return http.StatusBadRequest, errors.New(errMsg)
	}
	// validate user is allowed to create this user
	if payload.User_type != "member" {
		if payload.Signing_addr == payload.Addr {
			CANNOT_GRANT_SELF_ERR := errors.New("Users cannot grant themselves a priviledged user_type.")
			log.Error().Err(CANNOT_GRANT_SELF_ERR)
			return http.StatusForbidden, CANNOT_GRANT_SELF_ERR
		}
		// If signing address is not user address, verify they have admin status in this community
		var communityAdmin = models.CommunityUser{Community_id: payload.Community_id, Addr: payload.Signing_addr, User_type: "admin"}
		if err := communityAdmin.GetCommunityUser(h.A.DB); err != nil {
			USER_MUST_BE_ADMIN_ERR := errors.New("User must be community admin to grant priviledges.")
			log.Error().Err(err).Msg("Database error.")
			log.Error().Err(USER_MUST_BE_ADMIN_ERR)
			return http.StatusForbidden, USER_MUST_BE_ADMIN_ERR
		}
	}
	// only an account can add itself as a "member", unless an admin is granting
	// an address a privileged role
	if payload.User_type == "member" && payload.Addr != payload.Signing_addr {
		CANNOT_ADD_MEMBER_ERR := errors.New(
			"An account can only add itself as a community member, unless an admin is granting privileged role.",
		)
		log.Error().Err(CANNOT_ADD_MEMBER_ERR)
		return http.StatusForbidden, CANNOT_ADD_MEMBER_ERR
	}

	if err := h.validateUser(payload.Signing_addr, payload.Timestamp, payload.Composite_signatures); err != nil {
		log.Error().Err(err)
		return http.StatusForbidden, err
	}

	// check that community user doesnt already exist
	// should throw a "ErrNoRows" error
	u := payload.CommunityUser
	if err := u.GetCommunityUser(h.A.DB); err == nil {
		errMsg := fmt.Sprintf("Error: Address %s is already a %s of community %d.\n", u.Addr, u.User_type, u.Community_id)
		log.Error().Err(err).Msg(errMsg)
		return http.StatusBadRequest, errors.New(errMsg)
	}

	// Grant appropriate roles
	if u.User_type == "admin" {
		if err := models.GrantAdminRolesToAddress(h.A.DB, u.Community_id, u.Addr); err != nil {
			log.Error().Err(err)
			return http.StatusInternalServerError, err
		}
	} else if u.User_type == "author" {
		if err := models.GrantAuthorRolesToAddress(h.A.DB, u.Community_id, u.Addr); err != nil {
			return http.StatusInternalServerError, err
		}
	} else {
		// grant member role
		if err := u.CreateCommunityUser(h.A.DB); err != nil {
			log.Error().Err(err)
			return http.StatusInternalServerError, err
		}
	}

	return http.StatusCreated, nil
}

func (h *Helpers) updateAddressesInList(id int, payload models.ListUpdatePayload, action string) (int, error) {
	l := models.List{ID: id}

	// get current proposal from DB
	if err := l.GetListById(h.A.DB); err != nil {
		errMsg := fmt.Sprintf("Error querying list with id %v.", id)
		log.Error().Err(err).Msg(errMsg)
		return http.StatusInternalServerError, err
	}

	validate := validator.New()
	if vErr := validate.Struct(payload); vErr != nil {
		errMsg := "Remove from list validation error."
		if action == "add" {
			errMsg = "Add to list validation error."
		}
		log.Error().Err(vErr).Msg(errMsg)
		return http.StatusBadRequest, errors.New(errMsg)
	}

	if err := h.validateUserWithRole(payload.Signing_addr, payload.Timestamp, payload.Composite_signatures, l.Community_id, "admin"); err != nil {
		log.Error().Err(err)
		return http.StatusForbidden, err
	}

	if action == "remove" {
		l.RemoveAddresses(payload.Addresses)
	} else {
		l.AddAddresses(payload.Addresses)
	}

	cid, err := h.pinJSONToIpfs(l)
	if err != nil {
		log.Error().Err(err).Msg("IPFS error: " + err.Error())
		return http.StatusInternalServerError, errors.New("Error pinning JSON to IPFS.")
	}
	l.Cid = cid

	if err := l.UpdateList(h.A.DB); err != nil {
		errMsg := "Database error updating list."
		log.Error().Err(err).Msg(errMsg)
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}

func (h *Helpers) createListForCommunity(payload models.ListPayload) (models.List, int, error) {
	if existingList, _ := models.GetListForCommunityByType(h.A.DB, payload.Community_id, *payload.List_type); existingList.ID > 0 {
		errMsg := fmt.Sprintf("List of type %s already exists for community %d.", *payload.List_type, payload.Community_id)
		return models.List{}, http.StatusBadRequest, errors.New(errMsg)
	}

	// validate payload fields
	validate := validator.New()
	if vErr := validate.Struct(payload); vErr != nil {
		errMsg := "Validation error in list payload."
		log.Error().Err(vErr).Msg(errMsg)
		return models.List{}, http.StatusBadRequest, errors.New(errMsg)
	}

	if err := h.validateUserWithRole(payload.Signing_addr, payload.Timestamp, payload.Composite_signatures, payload.Community_id, "admin"); err != nil {
		log.Error().Err(err)
		return models.List{}, http.StatusForbidden, err
	}

	l := payload.List

	cid, err := h.pinJSONToIpfs(l)
	if err != nil {
		log.Error().Err(err).Msg("IPFS error: " + err.Error())
		return models.List{}, http.StatusInternalServerError, errors.New("Error pinning JSON to IPFS.")
	}
	l.Cid = cid

	// create list
	if err := l.CreateList(h.A.DB); err != nil {
		return models.List{}, http.StatusInternalServerError, err
	}

	return l, http.StatusCreated, nil
}

func (h *Helpers) validateUserSignature(addr string, message string, sigs *[]shared.CompositeSignature) error {
	shouldValidateSignature := h.A.Config.Features["validateSigs"]

	if !shouldValidateSignature {
		return nil
	}

	if err := h.A.FlowAdapter.ValidateSignature(addr, message, sigs, "USER"); err != nil {
		return err
	}
	return nil
}

func (h *Helpers) validateTxSignature(addr string, message string, sigs *[]shared.CompositeSignature) error {
	shouldValidateSignature := h.A.Config.Features["validateSigs"]

	if !shouldValidateSignature {
		return nil
	}

	if err := h.A.FlowAdapter.ValidateSignature(addr, message, sigs, "TRANSACTION"); err != nil {
		return err
	}
	return nil
}

func (h *Helpers) validateBlocklist(addr string, communityId int) error {
	if !h.A.Config.Features["validateBlocklist"] {
		return nil
	}

	blockList, _ := models.GetListForCommunityByType(h.A.DB, communityId, "block")
	isBlocked := funk.Contains(blockList.Addresses, addr)

	isTest := flag.Lookup("test.v") != nil

	if isBlocked && !isTest {
		return errors.New("User does not have permission.")
	}
	return nil
}

// Need to move this to conditional middleware
func (h *Helpers) validateTimestamp(timestamp string, expiry int) error {
	if !h.A.Config.Features["validateTimestamps"] {
		return nil
	}
	// check timestamp and ensure no longer than expiry seconds has passed
	stamp, _ := strconv.ParseInt(timestamp, 10, 64)
	uxTime := time.Unix(stamp/1000, (stamp%1000)*1000*1000)
	diff := time.Now().UTC().Sub(uxTime).Seconds()
	if diff > float64(expiry) {
		err := errors.New("Timestamp on request has expired.")
		log.Error().Err(err).Msgf("expiry error: %v", diff)
		return err
	}
	return nil
}

func (h *Helpers) validateUser(addr, timestamp string, compositeSignatures *[]shared.CompositeSignature) error {
	if err := h.validateTimestamp(timestamp, 60); err != nil {
		return err
	}
	if err := h.validateUserSignature(addr, timestamp, compositeSignatures); err != nil {
		return err
	}

	return nil
}

func (h *Helpers) validateUserWithRole(addr, timestamp string, compositeSignatures *[]shared.CompositeSignature, communityId int, role string) error {
	if err := h.validateTimestamp(timestamp, 60); err != nil {
		return err
	}
	if err := h.validateUserSignature(addr, timestamp, compositeSignatures); err != nil {
		return err
	}
	if err := models.EnsureRoleForCommunity(h.A.DB, addr, communityId, role); err != nil {
		errMsg := fmt.Sprintf("Account %s is not an author for community %d.", addr, communityId)
		log.Error().Err(err).Msg(errMsg)
		return err
	}

	return nil
}

func (h *Helpers) processSnapshotStatus(s *models.Strategy, p *models.Proposal) error {
	var processing = "processing"

	if s.Contract.Name != nil && p.Snapshot_status == &processing {
		snapshotResponse, err := h.A.SnapshotClient.
			GetSnapshotStatusAtBlockHeight(
				s.Contract,
				*p.Block_height,
			)
		if err != nil {
			return err
		}

		p.Snapshot_status = &snapshotResponse.Data.Status

		if err := p.UpdateSnapshotStatus(h.A.DB); err != nil {
			return err
		}
	}
	return nil

}

func (h *Helpers) processTokenThreshold(address string, s models.Strategy) (bool, error) {
	var scriptPath string
	stratName := *s.Name

	if stratName == "balance-of-nfts" {
		scriptPath = "./main/cadence/scripts/get_nfts_ids.cdc"
	} else {
		scriptPath = "./main/cadence/scripts/get_balance.cdc"

	}

	hasBalance, err := h.A.FlowAdapter.EnforceTokenThreshold(scriptPath, address, &s.Contract)
	if err != nil {
		return false, err
	}

	return hasBalance, nil
}

func (h *Helpers) initStrategy(name string) Strategy {
	s := strategyMap[name]
	if s == nil {
		return nil
	}

	s.InitStrategy(h.A.FlowAdapter, h.A.DB, h.A.SnapshotClient)

	return s
}

func (h *Helpers) pinJSONToIpfs(data interface{}) (*string, error) {
	pin, err := h.A.IpfsClient.PinJson(data)
	if err != nil {
		return nil, err
	}
	return &pin.IpfsHash, nil
}
