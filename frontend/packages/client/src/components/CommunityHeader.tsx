import Blockies from 'react-blockies';
import { JoinCommunityButton } from 'components';
import { useMediaQuery } from 'hooks';

interface Member {
  addr: string;
  communityId: number;
  isAdmin: boolean;
  isMember: boolean;
}
interface CommunityHeaderProps {
  isLoading?: boolean;
  id?: string;
  bannerImgUrl?: string;
  logo?: string;
  slug?: string;
  communityName?: string;
  members?: Member[];
  totalMembers: number;
  onLeaveCommunity?: any;
  onJoinCommunity?: any;
}

export default function CommunityHeader({
  isLoading = false,
  id,
  bannerImgUrl,
  logo,
  slug,
  communityName,
  members,
  totalMembers,
  onLeaveCommunity = () => {},
  onJoinCommunity = () => {},
}: CommunityHeaderProps): JSX.Element {
  const notMobile = useMediaQuery();
  console.log({ members });
  return (
    <div className="is-flex is-flex-direction-column">
      <div className="is-flex flex-1 is-justify-content-center commmunity-header-container">
        <div
          className="is-flex community-header-wrapper"
          style={{
            backgroundImage: bannerImgUrl ? `url(${bannerImgUrl})` : 'none',
            backgroundSize: 'cover',
            backgroundPosition: 'center',
            maxWidth: '1300px',
          }}
        />
      </div>
      <div className="is-flex section py-0">
        <div className="container">
          <div
            style={{
              position: 'absolute',
              top: notMobile ? '-75px' : '-68px',
              maxHeight: notMobile ? '149px' : '85px',
            }}
          >
            {isLoading && (
              <div
                role="img"
                aria-label="community banner"
                className="rounded-full"
                style={{
                  width: notMobile ? 149 : 85,
                  height: notMobile ? 149 : 85,
                  backgroundColor: '#F2F2F2',
                  border: '5px solid #F2F2F2',
                }}
              />
            )}
            {logo ? (
              <div
                role="img"
                aria-label="community banner"
                className="rounded-full"
                style={{
                  width: notMobile ? 149 : 85,
                  height: notMobile ? 149 : 85,
                  backgroundImage: `url(${logo})`,
                  backgroundRepeat: 'no-repeat',
                  backgroundPosition: 'center',
                  backgroundSize: 'cover',
                  border: '5px solid #F2F2F2',
                }}
              />
            ) : slug || id ? (
              <Blockies
                seed={slug ?? `seed-${id}`}
                size={10}
                scale={notMobile ? 14.9 : 8.5}
                className="blockies blockies-border"
              />
            ) : null}
          </div>
          <div
            className="columns pt-5 pt-0-mobile"
            style={{ marginTop: notMobile ? '75px' : '17px' }}
          >
            <div className="column pb-0-mobile">
              <h2 className="has-text-weight-bold is-size-3 is-size-4-mobile">
                {communityName}
              </h2>
              <p className="has-text-grey small-text mt-3 mt-1-mobile">
                {totalMembers} members
              </p>
              <div className="is-flex mt-3 mt-1-mobile">
                {members
                  ? members.map(({ addr }: Member, idx: number) => (
                      <div
                        key={`${idx}`}
                        className="blockies-wrapper is-relative"
                        style={{ right: `${idx * (notMobile ? 12 : 6)}px` }}
                      >
                        <Blockies
                          seed={addr}
                          size={notMobile ? 10 : 8.9}
                          scale={4}
                          className="blockies blockies-border"
                        />
                      </div>
                    ))
                  : null}
              </div>
            </div>
            <div className="column is-3 pb-0-mobile pt-2-mobile is-flex is-justify-content-end is-justify-content-start-mobile">
              <JoinCommunityButton
                communityId={id}
                onLeaveCommunity={onLeaveCommunity}
                onJoinCommunity={onJoinCommunity}
                size="large"
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
