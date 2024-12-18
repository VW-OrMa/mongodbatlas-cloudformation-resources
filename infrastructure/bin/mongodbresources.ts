#!/usr/bin/env node
import {MongoDbAtlasResourcesStack} from '../lib/mongo-db-atlas-resources-stack';
import {App, Environment} from "aws-cdk-lib";
import {VwsServiceProviderStack} from "../lib/vws-service-provider-stack";

const app = new App();
const env: Environment = {
  account: process.env.CDK_DEFAULT_ACCOUNT,
  region: process.env.CDK_DEFAULT_REGION
}

const mongoDbAtlasResourcesStack = new MongoDbAtlasResourcesStack(app, 'MongoDbAtlasResourcesStack', {
  stackName: 'MongoDbAtlasResources',
  env
});

const vwsServiceProviderStack = new VwsServiceProviderStack(app, 'MongoDBAtlasResourcesProviderStack', {
  stackName: 'MongoDBAtlasResourcesProvider',
  resourceBucket: mongoDbAtlasResourcesStack.customServiceProviderBucket,
  env
});
// Resources have to be available before the service provider can be created
vwsServiceProviderStack.addDependency(mongoDbAtlasResourcesStack);
