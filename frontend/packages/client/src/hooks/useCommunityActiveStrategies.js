import { useQuery } from 'react-query';
import { useErrorHandlerContext } from 'contexts/ErrorHandler';
import { fetchActiveStrategies } from 'api/community';

export default function useCommunityActiveVotingStrategies(communityId) {
  const { notifyError } = useErrorHandlerContext();
  const { isLoading, isError, data, error } = useQuery(
    ['active-strategies', communityId],
    async () => fetchActiveStrategies(communityId),
    {
      enabled: !!communityId,
    }
  );

  if (isError) {
    notifyError(error);
  }

  return {
    isLoading,
    isError,
    error,
    data: data ?? [],
  };
}
