import { useReducer, useCallback } from 'react';
import { defaultReducer, INITIAL_STATE } from '../reducers';
import { checkResponse, getCompositeSigs } from 'utils';
import { useErrorHandlerContext } from '../contexts/ErrorHandler';
import { useFileUploader } from 'hooks';

const setDefaultValue = (field, fallbackValue) => {
  if (field === undefined || field === '') {
    return fallbackValue;
  }
  return field;
};
export default function useCommunity() {
  const [state, dispatch] = useReducer(defaultReducer, {
    ...INITIAL_STATE,
    loading: false,
  });
  const { notifyError } = useErrorHandlerContext();
  // for now not using modal notification if there was an error uploading image
  const { uploadFile } = useFileUploader({ useModalNotifications: false });

  const getCommunities = useCallback(async () => {
    dispatch({ type: 'PROCESSING' });
    const url = `${process.env.REACT_APP_BACK_END_SERVER_API}/communities`;
    try {
      const response = await fetch(url);
      const communities = await checkResponse(response);
      dispatch({
        type: 'SUCCESS',
        payload: communities?.data ?? [],
      });
    } catch (err) {
      // notify user of error
      notifyError(err, url);
      dispatch({ type: 'ERROR', payload: { errorData: err.message } });
    }
  }, [dispatch, notifyError]);

  const createCommunity = useCallback(
    async (injectedProvider, communityData) => {
      dispatch({ type: 'PROCESSING' });
      const url = `${process.env.REACT_APP_BACK_END_SERVER_API}/communities`;
      try {
        const timestamp = Date.now().toString();
        const hexTime = Buffer.from(timestamp).toString('hex');
        const _compositeSignatures = await injectedProvider
          .currentUser()
          .signUserMessage(hexTime);

        const compositeSignatures = getCompositeSigs(_compositeSignatures);

        if (!compositeSignatures) {
          const statusText = 'No valid user signature found.';
          // modal error will open
          notifyError(
            {
              status: 'Something went wrong with creating the community.',
              statusText,
            },
            url
          );
          dispatch({
            type: 'ERROR',
            payload: { errorData: statusText },
          });
          return;
        }

        const {
          communityName: name,
          communityDescription: body,
          category,
          communityTerms: termsAndConditionsUrl,
          listAddrAdmins,
          listAddrAuthors,
          creatorAddr,
          slug,
          discordUrl,
          githubUrl,
          instagramUrl,
          twitterUrl,
          websiteUrl,
          logo,
          contractAdrress: contractAddr,
          contractName: contractN,
          storagePath: storageP,
          proposalThreshold: propThreshold,
          onlyAuthorsToSubmitProposals,
          strategies,
        } = communityData;

        let communityLogo;
        // not handling upload error: there's a default image
        // admins can edit later the image
        if (logo.file) {
          try {
            communityLogo = await uploadFile(logo.file);
          } catch (err) {
            communityLogo = undefined;
          }
        }

        const fetchOptions = {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            name,
            body,
            category,
            termsAndConditionsUrl,
            creatorAddr,
            additionalAuthors: listAddrAuthors?.map((ele) => ele.addr),
            additionalAdmins: listAddrAdmins?.map((ele) => ele.addr),
            slug,
            githubUrl,
            instagramUrl,
            twitterUrl,
            websiteUrl,
            discordUrl,
            logo: communityLogo?.fileUrl,
            contractAddress: setDefaultValue(
              contractAddr,
              '0x0ae53cb6e3f42a79'
            ),
            contractName: setDefaultValue(contractN, 'FlowToken'),
            storagePath: setDefaultValue(storageP, 'flowTokenBalance'),
            proposalThreshold: setDefaultValue(propThreshold, '0'),
            strategies,
            onlyAuthorsToSubmit: Boolean(onlyAuthorsToSubmitProposals),
            timestamp,
            compositeSignatures,
          }),
        };
        const response = await fetch(url, fetchOptions);
        const json = await checkResponse(response);
        dispatch({ type: 'SUCCESS', payload: json });
      } catch (err) {
        notifyError(err, url, 'Something went wrong with your proposal.');
        dispatch({ type: 'ERROR', payload: { errorData: err.message } });
      }
    },
    [dispatch, notifyError, uploadFile]
  );

  return {
    ...state,
    getCommunities,
    createCommunity,
  };
}
