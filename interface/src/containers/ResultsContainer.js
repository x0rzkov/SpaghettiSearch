import React, { Component } from 'react';
import Results from '../components/Results';
import history from '../utils/history';
import {Nav, Navbar, NavbarBrand, NavLink, NavItem, Form, Input} from 'reactstrap';
import '../styles/ResultsContainer.css';

class ResultsContainer extends Component {
  constructor (props) {
    super(props)
    this.resultsElement = React.createRef();
    if(typeof history.location.state !== 'undefined'){
      this.state = {query: history.location.state.query, results:[]};
    } else {
      this.state = {query: props.query, results:[]};
    }
    // this.state = {query: props.query, results: []}
  }
  componentDidMount (props) {
    if(typeof history.location.state !== 'undefined'){
      this.setState({query: history.location.state.query});
    } else {
      this.setState({query: this.props.query});
    }
  }
  handleChange = (ev) => {
    this.setState({
      query: ev.target.value
    });
  }
  getResults = (ev) => {
    ev.preventDefault();
    this.resultsElement.current.getResults(this.state.query);
  }
  render() {
    return (
      <div>
      <Navbar color="faded" light className="header" sticky={'top'}>
      <NavbarBrand href='/'><b>Spaghetti</b></NavbarBrand>
      <Nav className="mr-auto" navbar>
      <Form onSubmit={this.getResults}>
        <Input id='searchbar' type="search" className="searchbox--results" placeholder="What would you like to search?" defaultValue={this.state.query} onChange={this.handleChange}/>
      </Form>
      </Nav>
      </Navbar>
      <Results ref={this.resultsElement} query={this.state.query}/>
      </div>
    );
  }
}

export default ResultsContainer;
