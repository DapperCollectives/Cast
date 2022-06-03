import React, { useEffect, useCallback } from "react";
import { convertToRaw } from "draft-js";
import draftToHtml from "draftjs-to-html";
import { ProposalStatus, VoteOptions } from "../Proposal";
import { parseDateToServer } from "utils";
import { stateToHTML } from "draft-js-export-html";

const options = {
  blockRenderers: {
    "image-caption-block": (block) => {
      let data = block.getData();

      return "<p>" + block.getText() + "</p>";
    },
  },
};

const StepThree = ({ stepsData, setStepValid }) => {
  const setPreviewValid = useCallback(() => {
    setStepValid(true);
  }, [setStepValid]);

  useEffect(() => {
    setPreviewValid();
  }, [setPreviewValid]);

  const proposal = {
    endTime: parseDateToServer(stepsData[1].endDate, stepsData[1].endTime),
    startTime: parseDateToServer(
      stepsData[1].startDate,
      stepsData[1].startTime
    ),
    winCount: 0,
    choices: stepsData[0]?.choices
      ?.sort((a, b) => (a.value > b.value ? 1 : -1))
      .map((choice) => ({
        ...choice,
        label: choice.value,
      })),
  };

  // const rawContentState = convertToRaw(
  //   stepsData[0]?.description?.getCurrentContent()
  // );

  // const markup = draftToHtml(rawContentState);

  const markup = stateToHTML(
    stepsData[0]?.description?.getCurrentContent(),
    options
  );

  const htmlBody = markup
    .replace(/target="_self"/g, 'target="_blank" rel="noopener noreferrer"')
    .replace(/(?:\r\n|\r|\n)/g, "<br>");

  return (
    <div>
      <ProposalStatus proposal={proposal} />
      <h1 className="title mt-5 is-3">{stepsData[0]?.title}</h1>
      <div
        className="mt-6 mb-5 proposal-copy word-break-all"
        dangerouslySetInnerHTML={{
          __html: htmlBody,
        }}
      />
      <VoteOptions proposal={proposal} readOnly />
    </div>
  );
};

export default StepThree;
