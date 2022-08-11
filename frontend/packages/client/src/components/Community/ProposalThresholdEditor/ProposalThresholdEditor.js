import React from 'react';
import { useForm } from 'react-hook-form';
import { useWebContext } from 'contexts/Web3';
import { ActionButton, WrapperResponsive } from 'components';
import Checkbox from 'components/common/Checkbox';
import Input from 'components/common/Input';
import { yupResolver } from '@hookform/resolvers/yup';
import isEqual from 'lodash/isEqual';
import networks from 'networks';
import { Schema } from './FormConfig';

const networkConfig = networks[process.env.REACT_APP_FLOW_ENV];

const defaultVaules = {
  contractAddress: networkConfig.flowAddress.contractAddr,
  contractName: networkConfig.flowAddress.contractName,
  storagePath: networkConfig.flowAddress.storagePath,
  proposalThreshold: '0',
};

const checkFieldValues = ({
  contractAddress,
  contractName,
  storagePath,
  proposalThreshold,
  onlyAuthorsToSubmitProposals,
}) => {
  return (
    onlyAuthorsToSubmitProposals === true &&
    isEqual(defaultVaules, {
      contractAddress,
      contractName,
      storagePath,
      proposalThreshold,
    })
  );
};
export default function ProposalThresholdEditor({
  updateCommunity = () => {},
  contractAddress,
  contractName,
  storagePath,
  proposalThreshold,
  onlyAuthorsToSubmitProposals,
}) {
  const { isValidFlowAddress } = useWebContext();

  const useEmptyFields = checkFieldValues({
    contractAddress,
    contractName,
    storagePath,
    proposalThreshold,
    onlyAuthorsToSubmitProposals,
  });

  const { register, handleSubmit, formState, reset } = useForm({
    resolver: yupResolver(Schema(isValidFlowAddress)),
    defaultValues: {
      ...(useEmptyFields
        ? {
            proposalThreshold: '',
            contractAddress: '',
            contractName: '',
            storagePath: '',
          }
        : { proposalThreshold, contractAddress, contractName, storagePath }),
      onlyAuthorsToSubmitProposals,
    },
  });

  const onSubmit = async (data) => {
    await updateCommunity(data);
    reset(data, { keepDirty: false });
  };

  const { isDirty, isSubmitting, errors, isValid } = formState;

  return (
    <form onSubmit={handleSubmit(onSubmit)}>
      <WrapperResponsive
        classNames="border-light rounded-lg columns is-flex-direction-column is-mobile m-0"
        extraClasses="p-6 mb-5"
        extraClassesMobile="p-4 mb-4"
      >
        <div className="columns is-multiline">
          <div className="column is-12">
            <h4 className="has-text-weight-bold is-size-5">
              Proposal Threshold
            </h4>
          </div>
          <div className="column is-12">
            <p className="small-text has-text-grey">
              Proposal threshold is the minimum number of tokens community
              members must hold in order to create a proposal.
            </p>
          </div>
        </div>
        <Input
          placeholder="Contract Address"
          register={register}
          name="contractAddress"
          disabled={isSubmitting}
          error={errors['contractAddress']}
          classNames="rounded-sm border-light p-3 column is-full is-full-mobile mt-4"
        />
        <Input
          placeholder="Contract Name"
          register={register}
          name="contractName"
          disabled={isSubmitting}
          error={errors['contractName']}
          classNames="rounded-sm border-light p-3 column is-full is-full-mobile mt-4"
        />
        <Input
          placeholder="Collection Public Path"
          name="storagePath"
          register={register}
          disabled={isSubmitting}
          error={errors['storagePath']}
          classNames="rounded-sm border-light p-3 column is-full is-full-mobile mt-4"
        />
        <Input
          placeholder="Number of Tokens"
          name="proposalThreshold"
          register={register}
          disabled={isSubmitting}
          error={errors['proposalThreshold']}
          classNames="rounded-sm border-light p-3 column is-full is-full-mobile mt-4"
        />
        <Checkbox
          type="checkbox"
          name="onlyAuthorsToSubmitProposals"
          register={register}
          disabled={isSubmitting}
          error={errors['onlyAuthorsToSubmitProposals']}
          label="Allow only designated authors to submit proposals"
          labelClassNames="has-text-grey small-text"
        />
      </WrapperResponsive>
      <div className="columns mb-5">
        <div className="column is-12">
          <ActionButton
            type="submit"
            label="Next: VOTING STRATEGIES"
            enabled={isDirty && !isSubmitting}
            classNames="vote-button transition-all has-background-yellow mt-5"
          />
        </div>
      </div>
    </form>
  );
}
